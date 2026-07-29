// Generate pronounceable random names based on Adel I. Mirzazhanov's apg
package names

import (
	"fmt"
	"math/rand"
	"time"
)

type unit struct {
	unitCode string
	flags    uint16
}

const (
	notBeginSyllable uint16 = 0o10 // 8
	noFinalSplit     uint16 = 0o4  // 4
	vowel            uint16 = 0o2  // 2
	alternateVowel   uint16 = 0o1  // 1
	noSpecialRule    uint16 = 0
)

var rules = []unit{
	{"a", vowel},
	{"b", noSpecialRule},
	{"c", noSpecialRule},
	{"d", noSpecialRule},
	{"e", noFinalSplit | vowel},
	{"f", noSpecialRule},
	{"g", noSpecialRule},
	{"h", noSpecialRule},
	{"i", vowel},
	{"j", noSpecialRule},
	{"k", noSpecialRule},
	{"l", noSpecialRule},
	{"m", noSpecialRule},
	{"n", noSpecialRule},
	{"o", vowel},
	{"p", noSpecialRule},
	{"r", noSpecialRule},
	{"s", noSpecialRule},
	{"t", noSpecialRule},
	{"u", vowel},
	{"v", noSpecialRule},
	{"w", noSpecialRule},
	{"x", notBeginSyllable},
	{"y", alternateVowel | vowel},
	{"z", noSpecialRule},
	{"ch", noSpecialRule},
	{"gh", noSpecialRule},
	{"ph", noSpecialRule},
	{"rh", noSpecialRule},
	{"sh", noSpecialRule},
	{"th", noSpecialRule},
	{"wh", noSpecialRule},
	{"qu", noSpecialRule},
	{"ck", notBeginSyllable},
}

const (
	begin       int = 0o200 // 128
	notBegin    int = 0o100 // 64
	breakFlag   int = 0o40  // 32
	prefix      int = 0o20  // 16
	illegalPair int = 0o10  // 8
	suffix      int = 0o4   // 4
	end         int = 0o2   // 2
	notEnd      int = 0o1   // 1
	anyCombo    int = 0
)

// digram is a 34x34 linguistic transition matrix derived from the APG algorithm.
// It encodes phonetic pair-wise transition constraints and syllable boundaries between
// the 34 alphabet units defined in the rules array.
//
// Each row index i corresponds to a preceding unit, and each column index j corresponds
// to a succeeding unit. The bitwise mask at digram[i][j] controls phonetic relationships:
//   - illegalPair: unit j can never immediately follow unit i.
//   - begin: unit j starting a syllable is allowed here.
//   - notBegin: unit j cannot immediately start a syllable following unit i.
//   - breakFlag: a syllable division / boundary is required between unit i and unit j.
//   - prefix: unit j functions as a prefix relative to unit i.
//   - suffix: unit j functions as a suffix relative to unit i.
//   - end: sequence must end after unit j.
//   - notEnd: sequence cannot end at unit j.
//   - anyCombo: no special constraints or rules restrict this sequence.
var digram = [34][34]int{
	// 0: a
	{
		illegalPair,
		anyCombo,
		anyCombo,
		anyCombo,
		illegalPair,
		anyCombo,
		anyCombo,
		notBegin | breakFlag | notEnd,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		illegalPair,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		illegalPair,
		anyCombo,
		illegalPair,
		anyCombo,
		anyCombo,
		illegalPair,
		breakFlag | notEnd,
		anyCombo,
	},
	// 1: b
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		begin | suffix | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		begin | end,
		notBegin,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 2: c
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		suffix | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notEnd,
		notBegin | end,
		notBegin | prefix,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		anyCombo,
		notBegin | breakFlag | notEnd,
		illegalPair,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | suffix | notEnd,
		illegalPair,
	},
	// 3: d
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		begin | notEnd,
		notBegin | end,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | notEnd,
		notBegin | prefix,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 4: e
	{
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notEnd,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		breakFlag,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		notBegin | breakFlag | notEnd,
		anyCombo,
		illegalPair,
		anyCombo,
		anyCombo,
		illegalPair,
		breakFlag | notEnd,
		anyCombo,
	},
	// 5: f
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		begin | suffix | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		begin | notEnd,
		notBegin,
		notBegin,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 6: g
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		illegalPair,
		begin | suffix | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		begin | notEnd,
		notBegin | end,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin,
		notBegin,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 7: h
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 8: i
	{
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		notBegin,
		anyCombo,
		anyCombo,
		notBegin | breakFlag | notEnd,
		illegalPair,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		breakFlag,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		anyCombo,
		anyCombo,
		notBegin,
		anyCombo,
		illegalPair,
		anyCombo,
		anyCombo,
		illegalPair,
		breakFlag | notEnd,
		anyCombo,
	},
	// 9: j
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		anyCombo,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 10: k
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		suffix | notEnd,
		notBegin | breakFlag | notEnd,
		begin | suffix | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		suffix | notEnd,
		notBegin | end,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | prefix,
		illegalPair,
		notBegin,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 11: l
	{
		anyCombo,
		notBegin | prefix,
		notBegin | breakFlag | notEnd,
		notBegin | prefix,
		anyCombo,
		notBegin | prefix,
		notBegin | prefix,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | prefix,
		notBegin | prefix,
		notBegin | prefix,
		notBegin | prefix,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | prefix,
		notBegin | breakFlag | notEnd,
		notBegin,
		notBegin | prefix,
		anyCombo,
		notBegin | prefix,
		notBegin | breakFlag | notEnd,
		illegalPair,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | prefix,
		notBegin | breakFlag | notEnd,
		notBegin | prefix,
		illegalPair,
		notBegin | prefix,
		notBegin | prefix,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 12: m
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin,
		notBegin,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | prefix,
		notBegin | breakFlag | notEnd,
		notBegin,
		illegalPair,
		notBegin,
		notBegin,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 13: n
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | prefix,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | prefix,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin,
		notBegin,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | prefix,
		notBegin | breakFlag | notEnd,
		notBegin | prefix,
		illegalPair,
		notBegin,
		notBegin,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | prefix,
	},
	// 14: o
	{
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		illegalPair,
		anyCombo,
		anyCombo,
		notBegin | breakFlag | notEnd,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		notBegin,
		anyCombo,
		illegalPair,
		anyCombo,
		anyCombo,
		illegalPair,
		breakFlag | notEnd,
		anyCombo,
	},
	// 15: p
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		suffix | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | prefix,
		notEnd,
		notBegin | end,
		notBegin | end,
		notBegin | end,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 16: r
	{
		anyCombo,
		notBegin | prefix,
		notBegin | prefix,
		notBegin | prefix,
		anyCombo,
		notBegin | prefix,
		notBegin | prefix,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | prefix,
		notBegin | prefix,
		notBegin | prefix,
		notBegin | prefix,
		notBegin | prefix,
		anyCombo,
		notBegin | prefix,
		notBegin | prefix,
		notBegin | prefix,
		notBegin | prefix,
		anyCombo,
		notBegin | prefix,
		notBegin | breakFlag | notEnd,
		illegalPair,
		anyCombo,
		notBegin | prefix,
		notBegin | prefix,
		notBegin | breakFlag | notEnd,
		notBegin | prefix,
		illegalPair,
		notBegin | prefix,
		notBegin | prefix,
		illegalPair,
		notBegin | prefix | notEnd,
		notBegin | prefix,
	},
	// 17: s
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		anyCombo,
		begin | suffix | notEnd,
		suffix | notEnd,
		prefix | suffix | notEnd,
		anyCombo,
		anyCombo,
		notBegin | notEnd,
		notBegin | prefix,
		anyCombo,
		anyCombo,
		notBegin | breakFlag | notEnd,
		begin | suffix | notEnd,
		illegalPair,
		anyCombo,
		notBegin | breakFlag | notEnd,
		begin | suffix | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		suffix | notEnd,
		notBegin,
	},
	// 18: t
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notEnd,
		notBegin | end,
		notBegin | prefix,
		anyCombo,
		notBegin | breakFlag | notEnd,
		begin | suffix | notEnd,
		illegalPair,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | end,
		illegalPair,
		notBegin | end,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 19: u
	{
		notBegin | breakFlag | notEnd,
		anyCombo,
		anyCombo,
		anyCombo,
		notBegin,
		anyCombo,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		notBegin | breakFlag,
		anyCombo,
		anyCombo,
		anyCombo,
		anyCombo,
		illegalPair,
		anyCombo,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		anyCombo,
		anyCombo,
		notBegin | prefix,
		anyCombo,
		illegalPair,
		anyCombo,
		anyCombo,
		illegalPair,
		breakFlag | notEnd,
		anyCombo,
	},
	// 20: v
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 21: w
	{
		anyCombo,
		notBegin | prefix,
		notBegin | breakFlag | notEnd,
		notBegin | prefix | end,
		anyCombo,
		notBegin | prefix,
		notBegin | prefix | end,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | prefix,
		notBegin | prefix | suffix,
		notBegin | prefix,
		notBegin | prefix,
		anyCombo,
		notBegin | prefix,
		begin | suffix | notEnd,
		notBegin | prefix,
		notBegin | prefix,
		anyCombo,
		notBegin | prefix,
		notBegin | breakFlag | notEnd,
		notBegin | prefix,
		anyCombo,
		notBegin | prefix,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin,
		illegalPair,
		notBegin,
		notBegin,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin,
	},
	// 22: x
	{
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 23: y
	{
		anyCombo,
		notBegin,
		notBegin | notEnd,
		notBegin,
		anyCombo,
		notBegin | notEnd,
		notBegin,
		notBegin | breakFlag | notEnd,
		begin | notEnd,
		notBegin | notEnd,
		notBegin,
		notBegin | notEnd,
		notBegin,
		notBegin,
		anyCombo,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin,
		notBegin,
		anyCombo,
		notBegin | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin,
		illegalPair,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 24: z
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin,
		anyCombo,
		notBegin | breakFlag | notEnd,
		suffix | notEnd,
		illegalPair,
		anyCombo,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 25: ch
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | notEnd,
		illegalPair,
		anyCombo,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 26: gh
	{
		anyCombo,
		notBegin | breakFlag | prefix | notEnd,
		notBegin | breakFlag | prefix | notEnd,
		notBegin | breakFlag | prefix | notEnd,
		anyCombo,
		notBegin | breakFlag | prefix | notEnd,
		notBegin | breakFlag | prefix | notEnd,
		notBegin | breakFlag | prefix | notEnd,
		begin | notEnd,
		notBegin | breakFlag | prefix | notEnd,
		notBegin | breakFlag | prefix | notEnd,
		notBegin | breakFlag | prefix | notEnd,
		notBegin | breakFlag | prefix | notEnd,
		notBegin | breakFlag | prefix | notEnd,
		begin | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | prefix | notEnd,
		notBegin | prefix,
		notBegin | prefix,
		notBegin | breakFlag | prefix | notEnd,
		notBegin | breakFlag | prefix | notEnd,
		notBegin | breakFlag | prefix | notEnd,
		illegalPair,
		notBegin | breakFlag | prefix | notEnd,
		notBegin | breakFlag | prefix | notEnd,
		notBegin | breakFlag | prefix | notEnd,
		illegalPair,
		notBegin | breakFlag | prefix | notEnd,
		illegalPair,
		notBegin | breakFlag | prefix | notEnd,
		notBegin | breakFlag | prefix | notEnd,
		illegalPair,
		notBegin | breakFlag | prefix | notEnd,
		illegalPair,
	},
	// 27: ph
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		begin | suffix | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notEnd,
		notBegin,
		notBegin,
		anyCombo,
		notBegin | notEnd,
		notBegin | notEnd,
		illegalPair,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 28: rh
	{
		begin | notEnd,
		illegalPair,
		illegalPair,
		illegalPair,
		begin | notEnd,
		illegalPair,
		illegalPair,
		illegalPair,
		begin | notEnd,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		begin | notEnd,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		begin | notEnd,
		illegalPair,
		illegalPair,
		illegalPair,
		begin | notEnd,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
	},
	// 29: sh
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin,
		begin | suffix | notEnd,
		begin | suffix | notEnd,
		begin | suffix | notEnd,
		anyCombo,
		notBegin,
		begin | suffix | notEnd,
		notBegin | breakFlag | notEnd,
		suffix,
		anyCombo,
		notBegin | breakFlag | notEnd,
		suffix | notEnd,
		illegalPair,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 30: th
	{
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notEnd,
		notBegin | end,
		notBegin | breakFlag | notEnd,
		anyCombo,
		notBegin | breakFlag | notEnd,
		suffix | notEnd,
		illegalPair,
		anyCombo,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
	// 31: wh
	{
		begin | notEnd,
		illegalPair,
		illegalPair,
		illegalPair,
		begin | notEnd,
		illegalPair,
		illegalPair,
		illegalPair,
		begin | notEnd,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		begin | notEnd,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		begin | notEnd,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
	},
	// 32: qu
	{
		anyCombo,
		illegalPair,
		illegalPair,
		illegalPair,
		anyCombo,
		illegalPair,
		illegalPair,
		illegalPair,
		anyCombo,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		anyCombo,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
		illegalPair,
	},
	// 33: ck
	{
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		notBegin | breakFlag | notEnd,
		illegalPair,
		notBegin | breakFlag | notEnd,
		illegalPair,
	},
}

// numbers is a weighted frequency distribution array of rules indices for consonants and digraphs.
// It maps rules indices repeated in proportion to their natural usage frequency.
// For example, more common letters (like rules index 3 for "d") are repeated 12 times,
// whereas rare characters (like rules index 22 for "x") are included only once.
// This allows constant-time O(1) weighted random sampling of consonants using randomUnit().
var numbers = []uint16{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	1, 1, 1, 1, 1, 1, 1, 1,
	2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4,
	5, 5, 5, 5, 5, 5, 5, 5,
	6, 6, 6, 6, 6, 6, 6, 6,
	7, 7, 7, 7, 7, 7,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	9, 9, 9, 9, 9, 9, 9, 9,
	10, 10, 10, 10, 10, 10, 10, 10,
	11, 11, 11, 11, 11, 11,
	12, 12, 12, 12, 12, 12,
	13, 13, 13, 13, 13, 13, 13, 13, 13, 13,
	14, 14, 14, 14, 14, 14, 14, 14, 14, 14,
	15, 15, 15, 15, 15, 15,
	16, 16, 16, 16, 16, 16, 16, 16, 16, 16,
	17, 17, 17, 17, 17, 17, 17, 17,
	18, 18, 18, 18, 18, 18, 18, 18, 18, 18,
	19, 19, 19, 19, 19, 19,
	20, 20, 20, 20, 20, 20, 20, 20,
	21, 21, 21, 21, 21, 21, 21, 21,
	22,
	23, 23, 23, 23, 23, 23, 23, 23,
	24,
	25,
	26,
	27,
	28,
	29, 29,
	30,
	31,
	32,
	33,
}

// vowelNumbers is a weighted frequency distribution array of rules indices for vowels.
// It maps vowel indices (0: "a", 4: "e", 8: "i", 14: "o", 19: "u", 23: "y")
// repeated in proportion to their natural English usage frequency.
// This allows constant-time O(1) weighted random sampling of vowels using randomUnit().
var vowelNumbers = []uint16{
	0, 0, 4, 4, 4, 8, 8, 14, 14, 19, 19, 23,
}

type generatorState struct {
	savedUnit uint16
	savedPair [2]uint16
	rnd       *rand.Rand
}

func (state *generatorState) randomUnit(vowelExpected bool) uint16 {
	if vowelExpected {
		return vowelNumbers[state.rnd.Intn(len(vowelNumbers))]
	}
	return numbers[state.rnd.Intn(len(numbers))]
}

func (state *generatorState) genSyllable(nameLen int, unitsInSyllable []uint16) (string, uint16, error) {
	var syllable string
	var syllableLength uint16
	var ruleBroken bool

	holdSavedUnit := state.savedUnit

	for {
		tries := 0
		state.savedUnit = holdSavedUnit
		syllable = ""
		var vowelCount uint16 = 0
		var currentUnit int = 0
		lengthLeft := nameLen
		wantAnotherUnit := true

		for {
			wantVowel := false
			var chosenUnit uint16
			for {
				if state.savedUnit != 0 {
					if state.savedUnit == 2 {
						unitsInSyllable[0] = state.savedPair[1]
						if (rules[state.savedPair[1]].flags & vowel) != 0 {
							vowelCount++
						}
						currentUnit++
						syllable = rules[state.savedPair[1]].unitCode
						lengthLeft -= len(syllable)
					}
					chosenUnit = state.savedPair[0]
					state.savedUnit = 0
				} else {
					if wantVowel {
						chosenUnit = state.randomUnit(true)
					} else {
						chosenUnit = state.randomUnit(false)
					}
				}

				lengthLeft -= len(rules[chosenUnit].unitCode)
				if lengthLeft < 0 {
					ruleBroken = true
				} else {
					ruleBroken = false
				}

				if currentUnit == 0 {
					if (rules[chosenUnit].flags & notBeginSyllable) != 0 {
						ruleBroken = true
					} else {
						if lengthLeft == 0 {
							if (rules[chosenUnit].flags & vowel) != 0 {
								wantAnotherUnit = false
							} else {
								ruleBroken = true
							}
						}
					}
				} else {
					allowed := func(flag int) bool {
						return (digram[unitsInSyllable[currentUnit-1]][chosenUnit] & flag) != 0
					}

					if allowed(illegalPair) ||
						(allowed(breakFlag) && vowelCount == 0) ||
						(allowed(end) && vowelCount == 0 && (rules[chosenUnit].flags&vowel) == 0) {
						ruleBroken = true
					}

					if currentUnit == 1 {
						if allowed(notBegin) {
							ruleBroken = true
						}
					} else {
						lastUnit := unitsInSyllable[currentUnit-1]

						if (currentUnit == 2 && allowed(begin) && (rules[unitsInSyllable[0]].flags&alternateVowel) != 0) ||
							(allowed(notEnd) && lengthLeft == 0) ||
							(allowed(breakFlag) && (digram[unitsInSyllable[currentUnit-2]][lastUnit]&notEnd) != 0) ||
							(allowed(prefix) && (rules[unitsInSyllable[currentUnit-2]].flags&vowel) == 0) {
							ruleBroken = true
						}

						if !ruleBroken && (rules[chosenUnit].flags&vowel) != 0 &&
							(lengthLeft > 0 || (rules[lastUnit].flags&noFinalSplit) == 0) {
							if vowelCount > 1 && (rules[lastUnit].flags&vowel) != 0 {
								ruleBroken = true
							} else if vowelCount != 0 && (rules[lastUnit].flags&vowel) == 0 {
								if (digram[unitsInSyllable[currentUnit-2]][lastUnit] & notEnd) != 0 {
									ruleBroken = true
								} else {
									state.savedUnit = 1
									state.savedPair[0] = chosenUnit
									wantAnotherUnit = false
								}
							}
						}
					}

					if !ruleBroken && wantAnotherUnit {
						lastUnit := unitsInSyllable[currentUnit-1]
						if (vowelCount != 0 && (rules[chosenUnit].flags&noFinalSplit) != 0 && lengthLeft == 0 && (rules[lastUnit].flags&vowel) == 0) ||
							allowed(end) || lengthLeft == 0 {
							wantAnotherUnit = false
						} else if vowelCount != 0 && lengthLeft > 0 {
							if allowed(begin) && currentUnit > 1 &&
								!((vowelCount == 1) && (rules[lastUnit].flags&vowel) != 0) {
								state.savedUnit = 2
								state.savedPair[0] = chosenUnit
								state.savedPair[1] = lastUnit
								wantAnotherUnit = false
							} else if allowed(breakFlag) {
								state.savedUnit = 1
								state.savedPair[0] = chosenUnit
								wantAnotherUnit = false
							}
						} else if allowed(suffix) {
							wantVowel = true
						}
					}
				}

				tries++
				if ruleBroken {
					lengthLeft += len(rules[chosenUnit].unitCode)
				}

				if !ruleBroken || tries > 4*nameLen+34 {
					break
				}
			}

			if tries <= 4*nameLen+34 {
				if (rules[chosenUnit].flags&vowel) != 0 &&
					(currentUnit > 0 || (rules[chosenUnit].flags&alternateVowel) == 0) {
					vowelCount++
				}

				switch state.savedUnit {
				case 0:
					unitsInSyllable[currentUnit] = chosenUnit
					syllable += rules[chosenUnit].unitCode
				case 1:
					currentUnit--
				case 2:
					lastUnit := unitsInSyllable[currentUnit-1]
					syllable = syllable[:len(syllable)-len(rules[lastUnit].unitCode)]
					lengthLeft += len(rules[lastUnit].unitCode)
					currentUnit -= 2
				}
			} else {
				ruleBroken = true
			}

			syllableLength = uint16(currentUnit)
			currentUnit++

			if tries > 4*nameLen+34 || !wantAnotherUnit {
				break
			}
		}

		if !ruleBroken && !illegalPlacement(unitsInSyllable, int(syllableLength)) {
			break
		}
	}

	return syllable, syllableLength, nil
}

func improperWord(units []uint16, wordSize int) bool {
	for i := 0; i < wordSize; i++ {
		if i != 0 {
			u1 := units[i-1]
			u2 := units[i]
			if (digram[u1][u2] & illegalPair) != 0 {
				return true
			}
		}
		if i >= 2 {
			r2 := rules[units[i-2]].flags
			r1 := rules[units[i-1]].flags
			r0 := rules[units[i]].flags

			isVowel2 := (r2&vowel) != 0 && (r2&alternateVowel) == 0
			isVowel1 := (r1 & vowel) != 0
			isVowel0 := (r0 & vowel) != 0

			isCons2 := (r2 & vowel) == 0
			isCons1 := (r1 & vowel) == 0
			isCons0 := (r0 & vowel) == 0

			if (isVowel2 && isVowel1 && isVowel0) || (isCons2 && isCons1 && isCons0) {
				return true
			}
		}
	}
	return false
}

func haveInitialY(units []uint16, unitSize int) bool {
	vowelCount := 0
	normalVowelCount := 0

	for i := 0; i <= unitSize; i++ {
		r := rules[units[i]].flags
		if (r & vowel) != 0 {
			vowelCount++
			if (r&alternateVowel) == 0 || i != 0 {
				normalVowelCount++
			}
		}
	}
	return vowelCount <= 1 && normalVowelCount == 0
}

func haveFinalSplit(units []uint16, unitSize int) bool {
	vowelCount := 0
	for i := 0; i <= unitSize; i++ {
		if (rules[units[i]].flags & vowel) != 0 {
			vowelCount++
		}
	}
	return vowelCount == 1 && (rules[units[unitSize]].flags&noFinalSplit) != 0
}

func illegalPlacement(units []uint16, nameLen int) bool {
	vowelCount := 0
	for i := 0; i <= nameLen; i++ {
		if i >= 1 {
			cond1 := (rules[units[i-1]].flags&vowel) == 0 &&
				(rules[units[i]].flags&vowel) != 0 &&
				!((rules[units[i]].flags&noFinalSplit) != 0 && i == nameLen) &&
				vowelCount != 0

			cond2 := false
			if i >= 2 {
				r2 := rules[units[i-2]].flags
				r1 := rules[units[i-1]].flags
				r0 := rules[units[i]].flags

				isCons2 := (r2 & vowel) == 0
				isCons1 := (r1 & vowel) == 0
				isCons0 := (r0 & vowel) == 0

				isVowel2 := (r2&vowel) != 0 && !((rules[units[0]].flags&alternateVowel) != 0 && i == 2)
				isVowel1 := (r1 & vowel) != 0
				isVowel0 := (r0 & vowel) != 0

				if (isCons2 && isCons1 && isCons0) || (isVowel2 && isVowel1 && isVowel0) {
					cond2 = true
				}
			}
			if cond1 || cond2 {
				return true
			}
		}

		if (rules[units[i]].flags&vowel) != 0 &&
			!((rules[units[0]].flags&alternateVowel) != 0 && i == 0 && nameLen != 0) {
			vowelCount++
		}
	}
	return false
}

// GenNameOption defines a functional option configuration for GeneratePronounceableName.
type GenNameOption func(*genConfig)

// genConfig represents the internal generator configuration.
type genConfig struct {
	length int
	rnd    *rand.Rand
}

// WithLength returns a GenNameOption configuring the name length.
func WithLength(length int) GenNameOption {
	return func(cfg *genConfig) {
		cfg.length = length
	}
}

// WithSeed returns a GenNameOption that configures a custom seeded math/rand.Rand generator.
func WithSeed(seed int64) GenNameOption {
	return func(cfg *genConfig) {
		cfg.rnd = rand.New(rand.NewSource(seed))
	}
}

// WithRand returns a GenNameOption that configures a custom math/rand.Rand generator.
func WithRand(rnd *rand.Rand) GenNameOption {
	return func(cfg *genConfig) {
		cfg.rnd = rnd
	}
}

func genWord(cfg *genConfig) (string, error) {
	tries := 0
	wordLength := 0
	wordSize := 0

	wordUnits := make([]uint16, cfg.length+1)
	syllableUnits := make([]uint16, cfg.length+1)

	var word string

	state := &generatorState{
		rnd: cfg.rnd,
	}

	for wordLength < cfg.length {
		newSyllable, syllableSize, err := state.genSyllable(cfg.length-wordLength, syllableUnits)
		if err != nil {
			return "", err
		}
		syllableLength := len(newSyllable)

		for wordPlace := 0; wordPlace <= int(syllableSize); wordPlace++ {
			if wordSize+wordPlace < len(wordUnits) {
				wordUnits[wordSize+wordPlace] = syllableUnits[wordPlace]
			}
		}
		wordSize += int(syllableSize) + 1

		if improperWord(wordUnits[:wordSize], wordSize) ||
			(wordLength == 0 && haveInitialY(syllableUnits[:syllableSize+1], int(syllableSize))) ||
			(wordLength+syllableLength == cfg.length && haveFinalSplit(syllableUnits[:syllableSize+1], int(syllableSize))) {
			wordSize -= int(syllableSize) + 1
		} else {
			word += newSyllable
			wordLength += syllableLength
		}

		tries++
		if tries > 4*cfg.length+34 {
			wordLength = 0
			wordSize = 0
			tries = 0
			word = ""
		}
	}

	return word, nil
}

// GeneratePronounceableName generates a pronounceable random name of the specified length using the APG algorithm.
//
// By default, it generates a name of length 10. Pass GenNameOption to customize the generation:
//   - names.WithLength(int): specifies a length between 1 and 255.
//   - names.WithSeed(int64): configures a deterministic seeded pseudo-random generator.
//   - names.WithRand(*rand.Rand): configures a custom math/rand generator.
func GeneratePronounceableName(opts ...GenNameOption) (string, error) {
	cfg := &genConfig{
		length: 10,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.length <= 0 || cfg.length > 255 {
		return "", fmt.Errorf("invalid name length: %d (must be between 1 and 255)", cfg.length)
	}

	if cfg.rnd == nil {
		cfg.rnd = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	word, err := genWord(cfg)
	if err != nil {
		return "", err
	}
	return word, nil
}
