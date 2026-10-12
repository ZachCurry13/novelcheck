package store

import (
	"crypto/rand"
	"math/big"
	"strings"
)

// syncWords are short, plain words for sync codes: easy to read out and to
// type on an e-reader, with no look-alikes to mix up.
var syncWords = strings.Fields(`
able acorn actor agent alley amber angel apple april arrow aspen atlas aunt autumn bacon badge
baker bamboo banana barn basil beach beard bell berry bike bird blue boat bone book boot bowl
brave bread brick bridge brook broom brush bubble bucket cabin cable cactus cake camel candle
canoe canyon card carpet carrot castle cat cedar chair chalk cheese cherry chess chief chip
circle city clay clean clock cloud clover coast cobra cocoa coin comet copper coral corn cotton
cow crab crane cream crown crumb cup daisy dance dawn deer desk diamond dinner dock dog dolphin
door dove dragon dream drum duck eagle earth echo egg elbow elm ember engine fable falcon farm
feather fence fern field finch fire fish flag flame flower fog forest fox frog fruit garden
garlic geese ghost giant ginger glass globe glove goat gold goose grape grass green guitar
hammer harbor hat hawk hazel heart hedge hello hen herb hill honey horse house igloo ink iron
island ivory jacket jam jelly jewel juice jungle kayak kettle key kite kitten koala ladder lake
lamp lantern lark leaf lemon lilac lily lion llama lobster lunch magic mango maple marble
meadow melon mint mirror mitten moon moose mouse mud muffin napkin nest night noodle north nut
oak oat ocean olive onion orange otter owl oyster paint palm panda paper parrot peach pear
pebble pencil penny pepper piano pickle pie pilot pine pizza planet plum pocket pond poppy
potato puppy quail quilt rabbit radio rain raven river robin rocket rose rudder sail salad salt
sand sauce scarf seal shell ship shoe silver sky snow soup spoon spring squid star stone storm
straw sugar summer sun swan table tiger toast tomato tower train tree tulip turkey turtle
umbrella valley velvet violet walnut water wave whale wheat willow wind winter wolf wool yacht
yellow zebra
`)

// newSyncCode is three words run together, like "blueduckcheese".
func newSyncCode() string {
	var b strings.Builder
	for i := 0; i < 3; i++ {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(syncWords))))
		b.WriteString(syncWords[n.Int64()])
	}
	return b.String()
}
