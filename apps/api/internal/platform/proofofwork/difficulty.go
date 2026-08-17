package proofofwork

// Difficulty derives the cost of the next challenge from the rate limit bucket
// counter that already exists for the route.
//
// With a wall poster the invite identifier is the same for everyone, so the
// bucket degrades to (poster, /24). Tightening it would deny service to real
// guests: the pousada Wi-Fi puts every guest behind one address, and Brazilian
// mobile carriers use CGNAT, where a /24 can cover thousands of subscribers.
// Deriving the difficulty from the same counter gives the curve the feature
// actually wants — the first submission from a network is cheap, on a slow
// phone, and the tenth is expensive — instead of a 429 for the tenth guest.
func Difficulty(base, ceiling uint8, bitsPerStep, requestCount int32) uint8 {
	if !validDifficulty(base) || ceiling < base || !validDifficulty(ceiling) {
		return 0
	}
	return clamp(base, ceiling, base+step(bitsPerStep, requestCount))
}

// step is saturating on purpose: a counter large enough to overflow the
// addition would otherwise wrap and hand the abuser the cheapest challenge in
// the system, which is the opposite of what the counter is for.
func step(bitsPerStep, requestCount int32) uint8 {
	if bitsPerStep <= 0 || requestCount <= 0 {
		return 0
	}
	steps := requestCount / bitsPerStep
	if steps > MaxDifficultyBits {
		return MaxDifficultyBits
	}
	return uint8(steps)
}

func clamp(base, ceiling, value uint8) uint8 {
	if value < base {
		return ceiling
	}
	if value > ceiling {
		return ceiling
	}
	return value
}
