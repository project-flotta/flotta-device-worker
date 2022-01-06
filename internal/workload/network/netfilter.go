package network

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

const (
	nftcommand string = "nft"
	family     string = "inet" // for IPv4 and IPv6
)

type Error struct {
	exec.ExitError
	cmd        exec.Cmd
	msg        string
	exitStatus *int
}

func (e *Error) ExitStatus() int {
	if e.exitStatus != nil {
		return *e.exitStatus
	}
	return e.Sys().(syscall.WaitStatus).ExitStatus()
}

func (e *Error) Error() string {
	return fmt.Sprintf("running %v: exit status %v: %v", e.cmd.Args, e.ExitStatus(), e.msg)
}

//go:generate mockgen -package=network -destination=mock_netfilter.go . Netfilter
type Netfilter interface {
	AddTable(table string) error
	DeleteTable(table string) error
	AddChain(table, chain string) error
	DeleteChain(table, chain string) error
	AddRule(table, chain, rule string) error
}

func NewNetfilter() (Netfilter, error) {
	path, err := exec.LookPath(nftcommand)
	if err != nil {
		return nil, err
	}
	netfilter := &netfilter{
		path: path,
	}
	return netfilter, nil
}

type netfilter struct {
	path string
}

func (nft *netfilter) AddTable(table string) error {
	args := []string{"add", "table", family, table}
	return nft.run(args)
}

func (nft *netfilter) DeleteTable(table string) error {
	args := []string{"delete", "table", family, table}
	return nft.run(args)
}

func (nft *netfilter) AddChain(table, chain string) error {
	args := []string{"add", "chain", family, table, chain, "{ type filter hook input priority 0 ; }"}
	return nft.run(args)
}

func (nft *netfilter) DeleteChain(table, chain string) error {
	// verify chain existence before attempting to delete it
	args := []string{"list", "chain", family, table, chain}
	if err := nft.run(args); err != nil {
		v, ok := err.(*Error)
		if ok {
			if strings.Contains(v.msg, "No such file or directory") {
				return nil
			}
		}
		return err
	}

	args = []string{"delete", "chain", family, table, chain}
	return nft.run(args)
}

func (nft *netfilter) AddRule(table, chain, rule string) error {
	args := []string{"add", "rule", family, table, chain, rule}
	return nft.run(args)
}

func (nft *netfilter) run(args []string) error {
	args = append([]string{nft.path}, args...)
	var stderr bytes.Buffer
	cmd := exec.Cmd{
		Path:   nft.path,
		Args:   args,
		Stderr: &stderr,
	}

	if err := cmd.Run(); err != nil {
		switch e := err.(type) {
		case *exec.ExitError:
			return &Error{*e, cmd, stderr.String(), nil}
		default:
			return err
		}
	}

	return nil
}
