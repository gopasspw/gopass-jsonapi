package jsonapi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gopasspw/gopass/pkg/passkey"
	"github.com/twpayne/go-pinentry/v4"
)

const (
	// maxDisplayLen is the number of characters of a name that are shown to
	// the user.
	maxDisplayLen = 64
	// assuanNotConfirmed is the error code of a pinentry whose question was
	// answered with no.
	assuanNotConfirmed = 83886194
	// assuanTimeout is the error code of a pinentry that gave up waiting
	// for an answer.
	assuanTimeout = 83886142
)

// confirmer is the part of a pinentry client that is used to ask a question.
type confirmer interface {
	Confirm(option string) (bool, error)
	Close() error
}

// newPinentry starts a pinentry showing the given description. The pinentry
// is taken down when the context is done. It is a variable so that tests do
// not open dialogs.
var newPinentry = func(ctx context.Context, desc string) (confirmer, error) {
	proc := &pinentryProcess{
		done: ctx.Done(),
		command: func(name string, args []string) *exec.Cmd {
			return exec.CommandContext(ctx, name, args...)
		},
	}

	client, err := pinentry.NewClient(
		pinentry.WithBinaryNameFromGnuPGAgentConf(),
		pinentry.WithProcess(proc),
		pinentry.WithGPGTTY(),
		pinentry.WithTitle("gopass"),
		pinentry.WithDesc(desc),
		pinentry.WithOK("Confirm"),
		pinentry.WithCancel("Cancel"),
	)
	if err != nil {
		_ = proc.Close()

		return nil, err //nolint:wrapcheck
	}

	return &pinentryDialog{client: client, proc: proc}, nil
}

// pinentryDialog is a pinentry that is showing a question.
type pinentryDialog struct {
	client *pinentry.Client
	proc   *pinentryProcess
}

// Confirm waits for the answer of the user.
func (d *pinentryDialog) Confirm(option string) (bool, error) {
	return d.client.Confirm(option) //nolint:wrapcheck
}

// Close takes the dialog down. The pinentry is not asked to quit: nothing it
// could say matters anymore, and it must not be able to hold up the answer.
func (d *pinentryDialog) Close() error {
	return d.proc.Close()
}

// pinentryGrace is the time a pinentry is given to quit before it is killed.
const pinentryGrace = 2 * time.Second

// pinentryProcess runs a pinentry that is bound to a context. When the
// context is done the process is killed and the pipes to it are closed,
// which ends whatever is waiting for the pinentry.
type pinentryProcess struct {
	// done is closed when the pinentry is not needed anymore.
	done <-chan struct{}
	// command creates the command that is killed when done is closed.
	command func(name string, args []string) *exec.Cmd

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	lines  *bufio.Reader

	closing chan struct{}
	close   sync.Once
	err     error
}

// Start implements pinentry.Process.
func (p *pinentryProcess) Start(name string, args []string) error {
	cmd := p.command(name, args)
	// A program started by the pinentry, e.g. by a wrapper script, can keep
	// the pipes open after the pinentry is gone. Do not wait for it.
	cmd.WaitDelay = pinentryGrace

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to connect to pinentry: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()

		return fmt.Errorf("failed to connect to pinentry: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()

		// It names the program and is wrapped by pinentryConfirm.
		return err //nolint:wrapcheck
	}

	p.cmd, p.stdin, p.stdout = cmd, stdin, stdout
	p.lines = bufio.NewReader(stdout)
	p.closing = make(chan struct{})

	// Killing the process does not end a read from its output if something
	// else still has the pipe open. Closing this end of it does.
	go func() {
		select {
		case <-p.done:
			_ = stdout.Close()
			_ = stdin.Close()
		case <-p.closing:
		}
	}()

	return nil
}

// ReadLine implements pinentry.Process.
func (p *pinentryProcess) ReadLine() ([]byte, bool, error) {
	return p.lines.ReadLine()
}

// Write implements pinentry.Process.
func (p *pinentryProcess) Write(data []byte) (int, error) {
	return p.stdin.Write(data)
}

// Close implements pinentry.Process. It ends the pinentry and waits for it,
// but not for long: a pinentry that does not quit when its input ends is
// killed.
func (p *pinentryProcess) Close() error {
	p.close.Do(func() {
		if p.cmd == nil {
			return
		}

		close(p.closing)
		_ = p.stdin.Close()

		exited := make(chan error, 1)
		go func() { exited <- p.cmd.Wait() }()

		select {
		case p.err = <-exited:
		case <-time.After(pinentryGrace):
			_ = p.cmd.Process.Kill()
			p.err = <-exited
		}

		_ = p.stdout.Close()
	})

	return p.err
}

// pinentryConfirm asks the user to approve a WebAuthn ceremony with a
// pinentry dialog.
//
// The native messaging host has no terminal and the dialogs of the browser
// extension are not under its control. Asking with the pinentry the user
// already trusts with their passphrases makes sure that no passkey is created
// or used without the user noticing, no matter what a web page or the
// extension does.
func pinentryConfirm(ctx context.Context, req passkey.ConsentRequest) (bool, error) {
	desc, err := consentQuestion(req)
	if err != nil {
		return false, err
	}

	p, err := newPinentry(ctx, desc)
	if err != nil {
		return false, cannotAsk(fmt.Errorf("failed to start pinentry: %w", err))
	}

	confirmed, err := p.Confirm("")
	// Nothing depends on the pinentry anymore, whatever it says to goodbye.
	_ = p.Close()

	// A dialog that was taken down because the ceremony timed out or was
	// cancelled is no answer, even if the pinentry had one.
	if ctx.Err() != nil {
		return false, fmt.Errorf("no answer: %w", context.Cause(ctx))
	}

	if err != nil {
		var ae *pinentry.AssuanError

		switch {
		case pinentry.IsCancelled(err), errors.As(err, &ae) && ae.Code == assuanNotConfirmed:
			return false, nil
		case errors.As(err, &ae) && ae.Code == assuanTimeout:
			// The question was shown, nobody answered it.
			return false, fmt.Errorf("no answer: %w", err)
		default:
			// E.g. a pinentry for the terminal, which the browser does
			// not give its native messaging hosts.
			return false, cannotAsk(fmt.Errorf("pinentry failed: %w", err))
		}
	}

	return confirmed, nil
}

// cannotAsk marks a failure to put the question to the user. That is no
// reason to fail the request: the WebAuthn implementation of the browser
// can take it over, so it is reported as a fallback.
func cannotAsk(err error) error {
	return fmt.Errorf("%w: can not ask the user: %w", passkey.ErrUnsuitable, err)
}

// consentQuestion returns the question to ask the user for a ceremony.
func consentQuestion(req passkey.ConsentRequest) (string, error) {
	user := displayName(req.UserName)
	if user == `""` {
		user = displayName(req.UserDisplayName)
	}

	// The origins and the RP ID were validated. They contain nothing but
	// the characters of a URL.
	from := "Requested by " + req.Origin
	if req.TopOrigin != "" {
		from += ", embedded in " + req.TopOrigin
	}

	switch req.Operation {
	case passkey.OperationRegister:
		q := fmt.Sprintf("Create a passkey for %s at %s?", user, req.RPID)
		if req.Entry != "" {
			q = fmt.Sprintf("Create another passkey for %s at %s?\n\nThe account already has one in %s.", user, req.RPID, displayName(req.Entry))
		}

		return q + "\n\n" + from, nil
	case passkey.OperationAuthenticate:
		return fmt.Sprintf("Sign in to %s as %s?\n\nPasskey: %s\n%s", req.RPID, user, displayName(req.Entry), from), nil
	case passkey.OperationDisclose:
		return fmt.Sprintf("A passkey for %s at %s is already stored in %s.\n\nTell the site about it?\n\n%s", user, req.RPID, displayName(req.Entry), from), nil
	default:
		return "", fmt.Errorf("unknown operation %s", req.Operation)
	}
}

// displayName prepares a name chosen by a web site for being shown to the
// user. The site must not be able to add lines to the dialog or to make the
// name look like a part of the question, so the name is stripped of anything
// but printable characters, shortened and quoted.
func displayName(s string) string {
	s = strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || !unicode.IsPrint(r)
	}), " ")

	if r := []rune(s); len(r) > maxDisplayLen {
		s = string(r[:maxDisplayLen]) + "…"
	}

	return `"` + strings.ReplaceAll(s, `"`, `'`) + `"`
}
