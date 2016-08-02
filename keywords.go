package mtree

import (
	"fmt"
	"strings"
)

// Keyword is the string name of a keyword, with some convenience functions for
// determining whether it is a default or bsd standard keyword.
type Keyword string

// Default returns whether this keyword is in the default set of keywords
func (k Keyword) Default() bool {
	return inSlice(string(k), DefaultKeywords)
}

// Bsd returns whether this keyword is in the upstream FreeBSD mtree(8)
func (k Keyword) Bsd() bool {
	return inSlice(string(k), BsdKeywords)
}

// KeyVal is a "keyword=value"
type KeyVal string

// Keyword is the mapping to the available keywords
func (kv KeyVal) Keyword() string {
	if !strings.Contains(string(kv), "=") {
		return ""
	}
	chunks := strings.SplitN(strings.TrimSpace(string(kv)), "=", 2)[0]
	if !strings.Contains(chunks, ".") {
		return chunks
	}
	return strings.SplitN(chunks, ".", 2)[0]
}

// KeywordSuffix is really only used for xattr, as the keyword is a prefix to
// the xattr "namespace.key"
func (kv KeyVal) KeywordSuffix() string {
	if !strings.Contains(string(kv), "=") {
		return ""
	}
	chunks := strings.SplitN(strings.TrimSpace(string(kv)), "=", 2)[0]
	if !strings.Contains(chunks, ".") {
		return ""
	}
	return strings.SplitN(chunks, ".", 2)[1]
}

// Value is the data/value portion of "keyword=value"
func (kv KeyVal) Value() string {
	if !strings.Contains(string(kv), "=") {
		return ""
	}
	return strings.SplitN(strings.TrimSpace(string(kv)), "=", 2)[1]
}

// ChangeValue changes the value of a KeyVal
func (kv KeyVal) ChangeValue(newval string) string {
	return fmt.Sprintf("%s=%s", kv.Keyword(), newval)
}

// keywordSelector takes an array of "keyword=value" and filters out that only the set of words
func keywordSelector(keyval, words []string) []string {
	retList := []string{}
	for _, kv := range keyval {
		if inSlice(KeyVal(kv).Keyword(), words) {
			retList = append(retList, kv)
		}
	}
	return retList
}

// NewKeyVals constructs a list of KeyVal from the list of strings, like "keyword=value"
func NewKeyVals(keyvals []string) KeyVals {
	kvs := make(KeyVals, len(keyvals))
	for i := range keyvals {
		kvs[i] = KeyVal(keyvals[i])
	}
	return kvs
}

// KeyVals is a list of KeyVal
type KeyVals []KeyVal

// Has the "keyword" present in the list of KeyVal, and returns the
// corresponding KeyVal, else an empty string.
func (kvs KeyVals) Has(keyword string) KeyVal {
	for i := range kvs {
		if kvs[i].Keyword() == keyword {
			return kvs[i]
		}
	}
	return emptyKV
}

var emptyKV = KeyVal("")

// MergeSet takes the current setKeyVals, and then applies the entryKeyVals
// such that the entry's values win. The union is returned.
func MergeSet(setKeyVals, entryKeyVals []string) KeyVals {
	retList := NewKeyVals(append([]string{}, setKeyVals...))
	eKVs := NewKeyVals(entryKeyVals)
	seenKeywords := []string{}
	for i := range retList {
		word := retList[i].Keyword()
		if ekv := eKVs.Has(word); ekv != emptyKV {
			retList[i] = ekv
		}
		seenKeywords = append(seenKeywords, word)
	}
	for i := range eKVs {
		if !inSlice(eKVs[i].Keyword(), seenKeywords) {
			retList = append(retList, eKVs[i])
		}
	}
	return retList
}

var (
	// DefaultKeywords has the several default keyword producers (uid, gid,
	// mode, nlink, type, size, mtime)
	DefaultKeywords = []string{
		"size",
		"type",
		"uid",
		"gid",
		"mode",
		"link",
		"nlink",
		"time",
	}

	// DefaultTarKeywords has keywords that should be used when creating a manifest from
	// an archive. Currently, evaluating the # of hardlinks has not been implemented yet
	DefaultTarKeywords = []string{
		"size",
		"type",
		"uid",
		"gid",
		"mode",
		"link",
		"tar_time",
	}

	// BsdKeywords is the set of keywords that is only in the upstream FreeBSD mtree
	BsdKeywords = []string{
		"cksum",
		"device",
		"flags",
		"ignore",
		"gid",
		"gname",
		"link",
		"md5",
		"md5digest",
		"mode",
		"nlink",
		"nochange",
		"optional",
		"ripemd160digest",
		"rmd160",
		"rmd160digest",
		"sha1",
		"sha1digest",
		"sha256",
		"sha256digest",
		"sha384",
		"sha384digest",
		"sha512",
		"sha512digest",
		"size",
		"tags",
		"time",
		"type",
		"uid",
		"uname",
	}

	// SetKeywords is the default set of keywords calculated for a `/set` SpecialType
	SetKeywords = []string{
		"uid",
		"gid",
	}
)
