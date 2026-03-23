package main

import (
	"sync"

	"github.com/number571/go-peer/pkg/crypto/asymmetric"
)

type sChannel struct {
	chanID     string
	key        []byte
	aliasName  string
	pkHashes   []string
	pubKeysMap map[string]asymmetric.IPubKey
}

type sChannelsList struct {
	mu *sync.RWMutex
	m  map[string]struct{}
	l  []*sChannel
}

func newChannelsList() *sChannelsList {
	return &sChannelsList{
		mu: &sync.RWMutex{},
		m:  make(map[string]struct{}, 2048),
		l:  make([]*sChannel, 0, 2048),
	}
}

func (p *sChannelsList) addChannel(ch *sChannel) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.m[ch.chanID]; ok {
		return false
	}
	p.m[ch.chanID] = struct{}{}

	p.l = append(p.l, ch)
	return true
}

func (p *sChannelsList) getChannels() []*sChannel {
	p.mu.RLock()
	defer p.mu.RUnlock()

	r := make([]*sChannel, 0, len(p.l))
	r = append(r, p.l...)
	return r
}

func (p *sChannelsList) getLength() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.l)
}
