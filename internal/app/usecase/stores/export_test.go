package stores

// ClearCandidateKey empties the key of c, which a test does not compare.
func ClearCandidateKey(c *Candidate) { c.key = "" }

var ClearDir = clearDir

var NameCandidates = nameCandidates

var SetLater = setLater

var WarnPrefixDuplicates = warnPrefixDuplicates
