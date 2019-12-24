// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package circonus

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/rs/zerolog/log"
)

// Tag defines an individual tag
type Tag struct {
	Category string
	Value    string
}

// Tags defines a list of tags
type Tags []Tag

// TaggedName takes a string name and 0, 1, or 2 sets of tags.
//   * The first set of tags are the stream tags.
//   * The second set of tags are the measurement tags.
// NOTE: if there are no stream tags, but there are measurement
//       tags send an EMPTY set `[]string{}` for stream tags.
func (c *Check) taggedName(name string, tagSets ...[]string) string {
	metricName := name

	if len(tagSets[0]) > 0 {
		streamTags := make([]string, len(tagSets[0]))
		copy(streamTags, tagSets[0])
		sort.Strings(streamTags)
		metricName = fmt.Sprintf("%s|ST[%s]", metricName, encodeTags(streamTags, c.config.Base64Tags))
	}

	if len(tagSets) > 1 && len(tagSets[1]) > 0 {
		measurementTags := make([]string, len(tagSets[1]))
		copy(measurementTags, tagSets[1])
		sort.Strings(measurementTags)
		metricName = fmt.Sprintf("%s|MT[%s]", metricName, encodeTags(measurementTags, c.config.Base64Tags))
	}

	return metricName
}

func encodeTags(tags []string, useBase64 bool) string {
	if len(tags) == 0 {
		return ""
	}

	if !useBase64 {
		return strings.Join(tags, ",")
	}

	tagList := make([]string, len(tags))
	for i, tag := range tags {
		if i >= MaxTags {
			log.Warn().Int("num", len(tags)).Int("max", MaxTags).Interface("tags", tags).Msg("ignoring tags over max")
			break
		}

		tagParts := strings.SplitN(tag, ":", 2)
		if len(tagParts) != 2 {
			log.Warn().Str("tag", tag).Msg("stream tags must have a category and value, ignoring tag")
			continue // invalid tag, skip it
		}
		tc := tagParts[0]
		tv := tagParts[1]

		encodeFmt := `b"%s"`
		encodedSig := `b"` // has cat or val been previously (or manually) base64 encoded and formatted
		if !strings.HasPrefix(tc, encodedSig) {
			tc = fmt.Sprintf(encodeFmt, base64.StdEncoding.EncodeToString([]byte(strings.Map(removeSpaces, tc))))
		}
		if !strings.HasPrefix(tv, encodedSig) {
			tv = fmt.Sprintf(encodeFmt, base64.StdEncoding.EncodeToString([]byte(strings.Map(removeSpaces, tv))))
		}

		tagList[i] = tc + ":" + tv
	}

	return fmt.Sprintf("%q", strings.Join(tagList, ","))
}

func removeSpaces(r rune) rune {
	if unicode.IsSpace(r) {
		return -1
	}
	return r
}
