package main

import (
	"fmt"
	"github.com/je4/zsearch/v2/pkg/search"
)

func doStats(data []*search.SourceData) error {
	var items int64 = int64(len(data))
	var images, pdfs, videos, videoLength, audios, audioLength int64
	for _, item := range data {
		for t, ms := range item.GetMedia() {
			switch t {
			case "image":
				images += int64(len(ms))
			case "pdf":
				pdfs += int64(len(ms))
			case "video":
				videos += int64(len(ms))
				for _, m := range ms {
					videoLength += m.Duration
				}
			case "audio":
				audios += int64(len(ms))
				for _, m := range ms {
					audioLength += m.Duration
				}
			}

		}
	}
	fmt.Printf("Items: %v\n", items)
	fmt.Printf("Images: %v\n", images)
	fmt.Printf("PDFs: %v\n", pdfs)
	fmt.Printf("Videos: %v (%vsec)\n", videos, videoLength)
	fmt.Printf("Audios: %v (%vsec)\n", audios, audioLength)
	return nil
}
