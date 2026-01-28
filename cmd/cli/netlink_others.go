//go:build !linux

package cli

import "context"

func (p *Prog) watchLinkState(ctx context.Context) {}
