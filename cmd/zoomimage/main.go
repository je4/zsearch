package main

import (
	"emperror.dev/emperror"
	"emperror.dev/errors"
	"encoding/json"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/je4/utils/v2/pkg/zLogger"
	"github.com/je4/zsearch/v2/pkg/search"
	"github.com/rs/zerolog"
	"golang.org/x/image/draw"
	"image"
	"image/png"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const HEIGHT = 150

var mediaserverRegexp = regexp.MustCompile("^mediaserver:([^/]+)/([^/]+)$")

func main() {
	var err error
	cfgfile := flag.String("cfg", "./zoomimage.toml", "locations of config file")
	flag.Parse()
	config := LoadConfig(*cfgfile)

	//	HEIGHT := config.CHeight

	var out io.Writer = os.Stdout
	if config.Logfile != "" {
		fp, err := os.OpenFile(config.Logfile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("cannot open logfile %s: %v", config.Logfile, err)
		}
		defer fp.Close()
		out = fp
	}

	output := zerolog.ConsoleWriter{Out: out, TimeFormat: time.RFC3339}
	_logger := zerolog.New(output).With().Timestamp().Logger()
	_logger.Level(zLogger.LogLevel(config.Loglevel))
	var logger zLogger.ZLogger = &_logger

	mte, err := search.NewMTElasticSearch(config.ElasticSearch.Endpoint, config.ElasticSearch.Index, string(config.ElasticSearch.ApiKey), logger)
	if err != nil {
		logger.Panic().Err(err)
		return
	}

	//	"category:\"zotero2!!Institut Mode-Design Web\" catalog:\"iid\" catalog:\"Institut_IIG\" catalog:\"Institut_IMD\" catalog:\"hgkfotos\"",
	//	config.Filters
	scrollConfig := &search.ScrollConfig{
		Fields:         nil,
		QStr:           "",
		FiltersFields:  config.Filters,
		Groups:         []string{"global/guest"},
		ContentVisible: true,
		IsAdmin:        false,
	}

	var images = []struct {
		signature string
		img       image.Image
	}{}
	var width int64
	var cHeight = config.CHeight

	if err := mte.Scroll(scrollConfig, func(data *search.SourceData) error {
		if data.HasMedia == false {
			return nil
		}
		for mType, mediaList := range data.Media {
			for _, media := range mediaList {
				matches := mediaserverRegexp.FindStringSubmatch(media.Uri)
				if matches == nil {
					logger.Error().Msgf("invalid url format: %s", media.Uri)
					return errors.New(fmt.Sprintf("invalid url: %s", media.Uri))
				}
				collection := matches[1]
				signature := matches[2]
				logger.Info().Msgf("Loading %s", media.Uri)
				switch mType {
				case "image":
					if media.Mimetype == "image/x-canon-cr2" {
						logger.Warn().Msg("ignoring mime type image/x-canon-cr2")
						return nil
					}
				case "video":
					signature += "$$timeshot$$3"
				case "audio":
					signature += "$$poster"
				case "pdf":
					signature += "$$poster"
				default:
					logger.Warn().Msgf("invalid media type - %s", mType)
					return nil
				}
				msUrl := fmt.Sprintf("%s/%s/%s/resize/autorotate/formatpng/size%d0x%d", config.Mediaserver.Url, collection, signature, HEIGHT, HEIGHT)
				/*
					msUrl, err := ms.GetUrl(collection, signature, function)
					if err != nil {
						return errors.Wrapf(err, "cannot create url for %s/%s/%s", collection, signature, function)
					}
				*/
				logger.Info().Msgf("loading media: %s", msUrl)
				client := http.Client{
					Timeout: 3600 * time.Second,
				}
				resp, err := client.Get(msUrl)
				if err != nil {
					return errors.Wrapf(err, "cannot load url %s", msUrl)
				}
				defer resp.Body.Close()
				if resp.StatusCode >= 300 {
					logger.Error().Msgf("cannot get image: %v - %s", resp.StatusCode, resp.Status)
					//return errors.New(fmt.Sprintf("cannot get image: %v - %s", resp.StatusCode, resp.Status))
					return nil
				}
				img, _, err := image.Decode(resp.Body)
				if err != nil {
					logger.Error().Msgf("cannot decode image %s: %v", msUrl, err)
					return nil
				}
				dst := image.NewRGBA(image.Rect(0, 0, (cHeight*img.Bounds().Max.X)/img.Bounds().Max.Y, cHeight))
				draw.ApproxBiLinear.Scale(dst, dst.Rect, img, img.Bounds(), draw.Over, nil)
				images = append(images, struct {
					signature string
					img       image.Image
				}{signature: fmt.Sprintf("%s", data.Signature), img: dst})
				width += int64(img.Bounds().Dx())
			}
		}
		//		logger.Debug(data)
		return nil
	}); err != nil {
		logger.Panic().Err(err)
	}
	rand.Shuffle(len(images), func(i, j int) { images[i], images[j] = images[j], images[i] })

	intDx := config.Width
	intDy := config.Height
	coll := image.NewRGBA(image.Rectangle{
		Min: image.Point{},
		Max: image.Point{X: intDx, Y: intDy},
	})

	row := 0
	posX := 0
	positions := map[string][]image.Rectangle{}
	for i := 0; i < len(images); i++ {
		posY := row * cHeight
		key := i
		img := images[key]
		//	for key, img := range images {
		logger.Info().Msgf("collage image #%v of %v", key, len(images))
		draw.Copy(coll,
			image.Point{X: posX, Y: posY},
			img.img,
			img.img.Bounds(),
			draw.Over,
			nil)
		if _, ok := positions[img.signature]; !ok {
			positions[img.signature] = []image.Rectangle{}
		}
		positions[img.signature] = append(positions[img.signature], image.Rectangle{
			Min: image.Point{X: posX, Y: posY},
			Max: image.Point{X: posX + img.img.Bounds().Dx(), Y: posY + img.img.Bounds().Dy()},
		})
		posX += img.img.Bounds().Max.X
		if posX > intDx {
			posX = 0
			row++
			// repeat cropped image
			i--
		}
		if (row+1)*cHeight > intDy {
			logger.Info().Msgf("collage %v images of %v", key+1, len(images))
			break
		}
	}
	fp, err := os.Create(filepath.Join(config.ExportPath, "collage.png"))
	if err != nil {
		emperror.Panic(errors.Wrap(err, "cannot create collage file"))
	}
	if err := png.Encode(fp, coll); err != nil {
		fp.Close()
		emperror.Panic(errors.Wrap(err, "cannot encode collage png"))
	}
	fp.Close()

	fp, err = os.Create(filepath.Join(config.ExportPath, "collage.json"))
	if err != nil {
		emperror.Panic(errors.Wrap(err, "cannot create collage json file"))
	}
	jsonW := json.NewEncoder(fp)
	if err := jsonW.Encode(positions); err != nil {
		fp.Close()
		emperror.Panic(errors.Wrap(err, "cannot store json"))
	}
	fp.Close()
	fp, err = os.Create(filepath.Join(config.ExportPath, "collage.jsonl"))
	if err != nil {
		emperror.Panic(errors.Wrap(err, "cannot create collage jsonl file"))
	}
	for signature, rects := range positions {
		jsonBytes, err := json.Marshal(map[string]interface{}{
			"signature": signature,
			"rects":     rects,
		})
		if err != nil {
			fp.Close()
			emperror.Panic(errors.Wrap(err, "cannot store JSONL"))
		}
		jsonBytes = append(jsonBytes, []byte("\n")...)
		if _, err := fp.Write(jsonBytes); err != nil {
			fp.Close()
			emperror.Panic(errors.Wrap(err, "cannot store JSONL"))
		}
	}
	fp.Close()
	fp, err = os.Create(filepath.Join(config.ExportPath, "signatures.txt"))
	if err != nil {
		emperror.Panic(errors.Wrap(err, "cannot create signatures file"))
	}
	for signature, _ := range positions {
		str := signature + "\n"
		if _, err := fp.Write([]byte(str)); err != nil {
			fp.Close()
			emperror.Panic(errors.Wrap(err, "cannot store signatures"))
		}
	}
	fp.Close()

}
