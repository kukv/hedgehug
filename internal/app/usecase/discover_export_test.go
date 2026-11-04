package usecase

// ClearCandidateKey empties the key of c, which a test does not compare.
func ClearCandidateKey(c *Candidate) { c.key = "" }
