package main

import (
	"encoding/json"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"golang.org/x/image/draw"
	"image"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

const HEIGHT = 150

func collage(logger *logging.Logger, exportPath string, ms mediaserver.Mediaserver, data []*search.SourceData) error {
	var err error
	var images = []struct {
		signature string
		img       image.Image
	}{}

	var width int64

	logger.Infof("creating collage")

	for key, item := range data {
		logger.Infof("item %v of %v", key, len(data))
		derivatePath := filepath.ToSlash(filepath.Join("werke", item.SignatureOriginal, "derivate"))
		for _, medias := range item.GetMedia() {
			for _, m := range medias {
				var thumb string
				switch m.Type {
				case "video":
					thumb, err = mediaUrl(
						logger,
						exportPath,
						ms,
						derivatePath,
						"jpg",
						m.Uri+"$$timeshot$$3/resize/formatjpeg/size240x240")
				case "audio":
					thumb, err = mediaUrl(
						logger,
						exportPath,
						ms,
						derivatePath,
						"jpg",
						m.Uri+"$$poster/resize/formatjpeg/size240x240")
				case "pdf":
					thumb, err = mediaUrl(
						logger,
						exportPath,
						ms,
						derivatePath,
						"jpg",
						m.Uri+"$$poster/resize/formatjpeg/size240x240")
				case "image":
					thumb, err = mediaUrl(
						logger,
						exportPath,
						ms,
						derivatePath,
						"jpg",
						m.Uri+"/resize/autorotate/formatjpeg/size240x240")
				}
				if err != nil {
					logger.Error(err)
					continue
				}
				if thumb == "" {
					continue
				}
				fullpath := filepath.Join(exportPath, derivatePath, thumb)
				fp, err := os.Open(fullpath)
				if err != nil {
					logger.Panic(err)
				}
				img, _, err := image.Decode(fp)
				if err != nil {
					fp.Close()
					continue
				}
				fp.Close()
				dst := image.NewRGBA(image.Rect(0, 0, (HEIGHT*img.Bounds().Max.X)/img.Bounds().Max.Y, HEIGHT))
				draw.ApproxBiLinear.Scale(dst, dst.Rect, img, img.Bounds(), draw.Over, nil)
				images = append(images, struct {
					signature string
					img       image.Image
				}{signature: item.Signature, img: dst})
				width += int64(img.Bounds().Dx())
			}
		}
	}
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(images), func(i, j int) { images[i], images[j] = images[j], images[i] })
	dx := 118.9 * 118.110236220472 * 1.8 * 1.1
	intDx := int(dx)
	dy := 84.1 * 118.110236220472 * 1.8
	intDy := int(dy)
	coll := image.NewRGBA(image.Rectangle{
		Min: image.Point{},
		Max: image.Point{X: intDx, Y: intDy},
	})

	row := 0
	posX := 0
	positions := map[string][]image.Rectangle{}
	for i := 0; i < len(images); i++ {
		key := i
		img := images[key]
		//	for key, img := range images {
		logger.Infof("collage image #%v of %v", key, len(images))
		draw.Copy(coll,
			image.Point{X: posX, Y: row * HEIGHT},
			img.img,
			img.img.Bounds(),
			draw.Over,
			nil)
		if _, ok := positions[img.signature]; !ok {
			positions[img.signature] = []image.Rectangle{}
		}
		positions[img.signature] = append(positions[img.signature], image.Rectangle{
			Min: image.Point{X: posX, Y: row * HEIGHT},
			Max: image.Point{X: posX + img.img.Bounds().Dx(), Y: row*HEIGHT + img.img.Bounds().Dy()},
		})
		posX += img.img.Bounds().Max.X
		if posX > intDx {
			posX = 0
			row++
			// repeat cropped image
			i--
		}
		if (row+1)*HEIGHT > intDy {
			logger.Infof("collage %v images of %v", key, len(images))
			break
		}
	}
	fp, err := os.Create(filepath.Join(exportPath, "collage.png"))
	if err != nil {
		return errors.Wrapf(err, "cannot create collage file")
	}
	if err := png.Encode(fp, coll); err != nil {
		fp.Close()
		return errors.Wrapf(err, "cannot encode collage png")
	}
	fp.Close()

	fp, err = os.Create(filepath.Join(exportPath, "collage.json"))
	if err != nil {
		return errors.Wrapf(err, "cannot create collage json file")
	}
	jsonW := json.NewEncoder(fp)
	if err := jsonW.Encode(positions); err != nil {
		fp.Close()
		return errors.Wrapf(err, "cannot store json")
	}
	fp.Close()

	return nil
}
