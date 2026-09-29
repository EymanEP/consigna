// Package names generates friendly, pronounceable device names such as
// "quiet-otter" so people can tell devices apart without typing anything.
package names

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

var adjectives = []string{
	"amber", "bold", "brave", "bright", "calm", "clever", "cosmic", "crisp",
	"dapper", "eager", "fancy", "gentle", "glad", "golden", "happy", "hidden",
	"humble", "jolly", "kind", "lively", "lucky", "mellow", "misty", "noble",
	"polite", "proud", "quick", "quiet", "rapid", "rustic", "shy", "silent",
	"silver", "sleepy", "snowy", "solar", "steady", "still", "sunny", "swift",
	"tidy", "tiny", "vivid", "warm", "wild", "wise", "witty", "young",
}

var nouns = []string{
	"badger", "bear", "beaver", "bison", "crane", "crow", "deer", "dove",
	"eagle", "falcon", "ferret", "finch", "fox", "gecko", "hare", "hawk",
	"heron", "ibis", "koala", "lark", "lemur", "lynx", "marten", "mole",
	"moose", "newt", "orca", "otter", "owl", "panda", "pine", "puffin",
	"quail", "raven", "robin", "salmon", "seal", "sparrow", "stoat", "swan",
	"tapir", "tern", "toad", "trout", "walrus", "willow", "wolf", "wren",
}

// Generate returns a random name not rejected by taken. If every attempt
// collides (only possible with thousands of devices) a numeric suffix is
// added so the result is still unique.
func Generate(taken func(string) bool) string {
	for range 64 {
		name := pick(adjectives) + "-" + pick(nouns)
		if taken == nil || !taken(name) {
			return name
		}
	}
	base := pick(adjectives) + "-" + pick(nouns)
	for i := 2; ; i++ {
		name := fmt.Sprintf("%s-%d", base, i)
		if !taken(name) {
			return name
		}
	}
}

func pick(list []string) string {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(list))))
	if err != nil {
		// crypto/rand never fails on supported platforms.
		panic(err)
	}
	return list[n.Int64()]
}
