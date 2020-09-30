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

	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog/log"
)

// Tag defines an individual tag
type Tag struct {
	Category string
	Value    string
}

// Tags defines a list of tags
type Tags []Tag

func (c *Check) TagListToCGM(tags []string) cgm.Tags {
	tagList := make(cgm.Tags, len(tags))

	for i, tag := range tags {
		if i >= MaxTags {
			log.Warn().Int("num", len(tags)).Int("max", MaxTags).Interface("tags", tags).Msg("ignoring tags over max")
			break
		}
		if tag == "" {
			continue
		}

		if !strings.Contains(tag, ":") {
			tagList[i] = cgm.Tag{Category: tag, Value: ""}
			continue
		}

		tagParts := strings.SplitN(tag, ":", 2)
		if len(tagParts) != 2 {
			tagList[i] = cgm.Tag{Category: tag, Value: ""}
			continue
		}

		tagList[i] = cgm.Tag{Category: tagParts[0], Value: tagParts[1]}
	}

	return tagList
}

func (c *Check) NewTagList(tagSets ...[]string) []string {
	totTags := 0
	if len(tagSets) == 0 {
		return []string{}
	}
	for i := 0; i < len(tagSets); i++ {
		totTags += len(tagSets[i])
	}

	tagList := make([]string, totTags)
	idx := 0
	for i := 0; i < len(tagSets); i++ {
		for j := 0; j < len(tagSets[i]); j++ {
			tagList[idx] = tagSets[i][j]
			idx++
		}
	}

	sort.Strings(tagList)

	return tagList
}

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
		tagList := encodeTags(streamTags, c.config.Base64Tags)
		if tagList != "" {
			metricName = fmt.Sprintf("%s|ST[%s]", metricName, tagList)
		}
	}

	if len(tagSets) > 1 && len(tagSets[1]) > 0 {
		measurementTags := make([]string, len(tagSets[1]))
		copy(measurementTags, tagSets[1])
		sort.Strings(measurementTags)
		tagList := encodeTags(measurementTags, c.config.Base64Tags)
		if tagList != "" {
			metricName = fmt.Sprintf("%s|MT[%s]", metricName, tagList)
		}
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

	var tagList []string //nolint:prealloc
	for i, tag := range tags {
		if i >= MaxTags {
			log.Warn().Int("num", len(tags)).Int("max", MaxTags).Interface("tags", tags).Msg("ignoring tags over max")
			break
		}

		if tag == "" {
			continue
		}

		tc := ""
		tv := ""

		if strings.Contains(tag, ":") {
			tagParts := strings.SplitN(tag, ":", 2)
			if len(tagParts) != 2 {
				log.Warn().Str("tag", tag).Msg("stream tags must have a category and value, ignoring tag")
				continue // invalid tag, skip it
			}
			tc = tagParts[0]
			tv = tagParts[1]
		} else {
			tc = tag
		}

		if len(tc) > MaxTagCat {
			log.Warn().Str("tag", tag).Msgf("tag category longer than %d", MaxTagCat)
			continue
		} else if len(tc)+len(tv) > MaxTagLen {
			log.Warn().Str("tag", tag).Msgf("tag length longer than %d", MaxTagLen)
			continue
		}

		encodeFmt := `b"%s"`
		encodedSig := `b"` // has cat or val been previously (or manually) base64 encoded and formatted

		tg := ""
		if !strings.HasPrefix(tc, encodedSig) {
			tc = fmt.Sprintf(encodeFmt, base64.StdEncoding.EncodeToString([]byte(strings.Map(removeSpaces, strings.ToLower(tc)))))
		}
		tg += tc
		tg += ":" // always add the colon, so category only tags will work (the metric is rejected w/o the colon)

		if tv != "" {
			if !strings.HasPrefix(tv, encodedSig) {
				tv = fmt.Sprintf(encodeFmt, base64.StdEncoding.EncodeToString([]byte(strings.Map(removeSpaces, tv))))
			}
			tg += tv
		}

		tagList = append(tagList, tg)
	}

	return strings.Join(tagList, ",")
}

func removeSpaces(r rune) rune {
	if unicode.IsSpace(r) {
		return -1
	}
	return r
}
