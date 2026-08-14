package storjstatsreceiver

import "bytes"

// statKey is the decoded form of an admproto metric key.
// Wire format: "base_name,tag1=val1,tag2=val2 field".
// Either the tag list or the field may be absent.
type statKey struct {
	name  string
	tags  map[string]string
	field string
}

// parseStatKey splits the admproto key into its base name, tag pairs, and
// trailing field. Tag pairs missing an '=' are silently dropped. Callers own
// no reference to the input slice's memory in the returned struct — all
// strings are newly allocated.
func parseStatKey(key []byte) statKey {
	nameAndTags := key
	var field string
	if sp := bytes.IndexByte(key, ' '); sp >= 0 {
		nameAndTags = key[:sp]
		field = string(key[sp+1:])
	}

	tags := map[string]string{}
	name := nameAndTags
	if comma := bytes.IndexByte(nameAndTags, ','); comma >= 0 {
		name = nameAndTags[:comma]
		rest := nameAndTags[comma+1:]
		for len(rest) > 0 {
			var pair []byte
			if c := bytes.IndexByte(rest, ','); c >= 0 {
				pair = rest[:c]
				rest = rest[c+1:]
			} else {
				pair = rest
				rest = nil
			}
			if eq := bytes.IndexByte(pair, '='); eq > 0 {
				tags[string(pair[:eq])] = string(pair[eq+1:])
			}
		}
	}

	return statKey{
		name:  string(name),
		tags:  tags,
		field: field,
	}
}
