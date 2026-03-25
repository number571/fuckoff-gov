package main

import (
	"slices"
	"sync"
	"time"

	"github.com/number571/go-peer/pkg/crypto/asymmetric"
)

type sChannel struct {
	isFavorite bool
	timeAdd    time.Time
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

func (p *sChannelsList) sortChannels() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p._sortChannels()
}

func (p *sChannelsList) addChannel(ch *sChannel) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.m[ch.chanID]; ok {
		return false
	}
	p.m[ch.chanID] = struct{}{}

	p.l = append(p.l, ch)
	p._sortChannels()

	return true
}

func (p *sChannelsList) delChannel(chanID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.m[chanID]; !ok {
		return false
	}

	for i, v := range p.l {
		if v.chanID == chanID {
			p.l = append(p.l[:i], p.l[i+1:]...)
			delete(p.m, chanID)
			break
		}
	}

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

func (p *sChannelsList) _sortChannels() {
	p._sortByTimeAdd()
	p._sortByFavorites()
}

func (p *sChannelsList) _sortByTimeAdd() {
	slices.SortFunc(p.l, func(a, b *sChannel) int {
		if a.timeAdd.After(b.timeAdd) {
			return 1
		}
		if a.timeAdd.Before(b.timeAdd) {
			return -1
		}
		return 0
	})
}

func (p *sChannelsList) _sortByFavorites() {
	slices.SortFunc(p.l, func(a, b *sChannel) int {
		if !a.isFavorite && b.isFavorite {
			return 1
		}
		if a.isFavorite && !b.isFavorite {
			return -1
		}
		return 0
	})
}
