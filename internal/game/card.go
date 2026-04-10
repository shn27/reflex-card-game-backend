package game

import "math/rand"

var ranks = []string{"A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K"}
var suits = []string{"hearts", "diamonds", "clubs", "spades"}

// Card represents a single playing card.
type Card struct {
	Rank string `json:"rank"`
	Suit string `json:"suit"`
}

func (c Card) IsAce() bool {
	return c.Rank == "A"
}

// NewShuffledDeck returns a freshly shuffled standard 52-card deck.
func NewShuffledDeck() []Card {
	deck := make([]Card, 0, 52)
	for _, suit := range suits {
		for _, rank := range ranks {
			deck = append(deck, Card{Rank: rank, Suit: suit})
		}
	}
	rand.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})
	return deck
}
