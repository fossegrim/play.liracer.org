package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/fossegrim/play.liracer.org/snippet"
	"github.com/gorilla/websocket"
)

type room struct {
	sync.Mutex
	// players is the set of players currently in room.
	players map[*player]struct{}
	snippet snippet.Snippet
	roundId roundId
}

type roundId int

// newRoom creates a new room with a random snippet.
func newRoom() *room {
	return &room{
		players: make(map[*player]struct{}),
		snippet: snippet.Random(),
		roundId: 1,
	}
}

// CONCURRENCY_UNSAFE_sendTo sends bs to p. If an error occurs p it is logged.
// CONCURRENCY_UNSAFE_sendTo is not concurrency safe.
func (r *room) sendTo(p *player, bs []byte) {
	err := p.WriteMessage(websocket.TextMessage, bs)
	if err != nil {
		log.Printf("room: write to %v failed: %s\n", p, err)
		return
	}
	log.Printf("room: wrote to %v %q\n", p, bs)
}

func (r *room) handlePlayerTypedCorrectChars(p *player, correctChars int) {
	r.Lock()
	defer r.Unlock()
	if correctChars == len(r.snippet.Code) {
		snip := snippet.Random()
		r.snippet = snip
		oldId := r.roundId
		r.roundId++

		bs, err := json.Marshal(outgoingMsg{
			NewRoundMsg: &NewRoundOutgoingMsg{
				Snippet:    r.snippet.Code,
				NewRoundId: r.roundId,
				RoundId:    oldId,
			},
		})
		if err != nil {
			panic("marshalling a outgoingMsg should never result in an error")
		}
		for pp := range r.players {
			r.sendTo(pp, bs)
		}

		bs, err = json.Marshal(outgoingMsg{
			ChatMessageMsg: &ChatMessageOutgoingMsg{
				Sender:  "liracer",
				Content: fmt.Sprintf("%s won the round!", p),
			},
		})
		if err != nil {
			panic("marshalling a outgoingMsg should never result in an error")
		}
		for pp := range r.players {
			r.sendTo(pp, bs)
		}
		return
	}
	bs, err := json.Marshal(
		outgoingMsg{
			OpponentCorrectCharsMsg: &OpponentCorrectCharsOutgoingMsg{
				OpponentID:   p.id,
				CorrectChars: correctChars,
				RoundId:      r.roundId,
			},
		},
	)
	if err != nil {
		panic("marshalling a outgoingMsg should never result in an error")
	}
	for pp := range r.players {
		if pp != p {
			r.sendTo(pp, bs)
		}
	}
}

func (r *room) handlePlayerSentChatMessage(p *player, content string) {
	r.Lock()
	defer r.Unlock()

	bs, err := json.Marshal(outgoingMsg{
		ChatMessageMsg: &ChatMessageOutgoingMsg{
			Content: content,
			Sender:  p.String(),
		},
	})
	if err != nil {
		panic("marshalling a outgoingMsg should never result in an error")
	}

	for p := range r.players {
		r.sendTo(p, bs)
	}
}

func (r *room) handlePlayerJoined(p *player) {
	r.Lock()
	defer r.Unlock()

	r.players[p] = struct{}{}
	log.Printf("room: %v joined, there are now %d players\n", p, len(r.players))

	bs, err := json.Marshal(
		outgoingMsg{
			NewRoundMsg: &NewRoundOutgoingMsg{
				Snippet:    r.snippet.Code,
				NewRoundId: r.roundId,
				RoundId:    0,
			},
		},
	)
	if err != nil {
		panic("marshalling a outgoingMsg should never result in an error")
	}
	r.sendTo(p, bs)

	for pp := range r.players {
		bs, err := json.Marshal(
			outgoingMsg{
				ChatMessageMsg: &ChatMessageOutgoingMsg{
					Content: fmt.Sprintf("%s joined the room", p),
					Sender:  "liracer",
				},
			},
		)
		if err != nil {
			panic("marshalling a outgoingMsg should never result in an error")
		}
		r.sendTo(pp, bs)
	}
}

func (r *room) handlePlayerLeft(p *player) {
	r.Lock()
	defer r.Unlock()

	delete(r.players, p)
	log.Printf("room: %v left, there are now %d players\n", p, len(r.players))

	for pp := range r.players {
		bs, err := json.Marshal(
			outgoingMsg{
				ChatMessageMsg: &ChatMessageOutgoingMsg{
					Content: fmt.Sprintf("%s left the room", p),
					Sender:  "liracer",
				},
			},
		)
		if err != nil {
			panic("marshalling a outgoingMsg should never result in an error")
		}
		r.sendTo(pp, bs)
	}
}
