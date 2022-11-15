package main

import (
	"database/sql"
	"emperror.dev/emperror"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/je4/utils/v2/pkg/ssh"
	"github.com/je4/zsearch/v2/pkg/mediaserver"
	"github.com/je4/zsearch/v2/pkg/search"
	"golang.org/x/image/draw"
	"image"
	"image/png"
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

	// create logger instance
	logger, lf := search.CreateLogger("zoomimage", config.Logfile, config.Loglevel)
	defer lf.Close()

	var tunnels []*ssh.SSHtunnel
	for name, tunnel := range config.Tunnel {
		logger.Infof("starting tunnel %s", name)

		forwards := make(map[string]*ssh.SourceDestination)
		for fwName, fw := range tunnel.Forward {
			forwards[fwName] = &ssh.SourceDestination{
				Local: &ssh.Endpoint{
					Host: fw.Local.Host,
					Port: fw.Local.Port,
				},
				Remote: &ssh.Endpoint{
					Host: fw.Remote.Host,
					Port: fw.Remote.Port,
				},
			}
		}

		t, err := ssh.NewSSHTunnel(
			tunnel.User,
			tunnel.PrivateKey,
			&ssh.Endpoint{
				Host: tunnel.Endpoint.Host,
				Port: tunnel.Endpoint.Port,
			},
			forwards,
			logger,
		)
		if err != nil {
			logger.Errorf("cannot create tunnel %v@%v:%v - %v", tunnel.User, tunnel.Endpoint.Host, tunnel.Endpoint.Port, err)
			return
		}
		if err := t.Start(); err != nil {
			logger.Errorf("cannot create configfile %v - %v", t.String(), err)
			return
		}
		tunnels = append(tunnels, t)
	}
	defer func() {
		for _, t := range tunnels {
			t.Close()
		}
	}()
	// if tunnels are made, wait until connection is established
	if len(config.Tunnel) > 0 {
		time.Sleep(2 * time.Second)
	}

	mediadb, err := sql.Open(config.Mediaserver.DB.ServerType, config.Mediaserver.DB.DSN)
	if err != nil {
		logger.Panic(err)
		return
	}
	defer mediadb.Close()
	err = mediadb.Ping()
	if err != nil {
		logger.Panic(err)
		return
	}

	ms, err := mediaserver.NewMediaserverMySQL(config.Mediaserver.Url, mediadb, config.Mediaserver.DB.Schema, logger)
	if err != nil {
		logger.Panic(err)
		return
	}

	mte, err := search.NewMTElasticSearch(config.ElasticSearch.Endpoint, config.ElasticSearch.Index, logger)
	if err != nil {
		logger.Panic(err)
		return
	}

	//	"category:\"zotero2!!Institut Mode-Design Web\" catalog:\"iid\" catalog:\"Institut_IIG\" catalog:\"Institut_IMD\" catalog:\"hgkfotos\"",
	//	config.Filters
	scrollConfig := &search.ScrollConfig{
		Fields:         nil,
		QStr:           "",
		FiltersFields:  config.Filters,
		Groups:         []string{"global/user", "global/admin"},
		ContentVisible: false,
		IsAdmin:        true,
	}

	var images = []struct {
		url string
		img image.Image
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
					logger.Errorf("invalid url format: %s", media.Uri)
					return errors.New(fmt.Sprintf("invalid url: %s", media.Uri))
				}
				collection := matches[1]
				signature := matches[2]
				logger.Infof("Loading %s", media.Uri)
				switch mType {
				case "image":
					if media.Mimetype == "image/x-canon-cr2" {
						logger.Warning("ignoring mime type image/x-canon-cr2")
						return nil
					}
				case "video":
					signature += "$$timeshot$$3"
				case "audio":
					signature += "$$poster"
				case "pdf":
					signature += "$$poster"
				default:
					logger.Warningf("invalid media type - %s", mType)
					return nil
				}
				function := fmt.Sprintf("resize/autorotate/formatpng/size%d0x%d", HEIGHT, HEIGHT)
				msUrl, err := ms.GetUrl(collection, signature, function)
				if err != nil {
					return emperror.Wrapf(err, "cannot create url for %s/%s/%s", collection, signature, function)
				}
				logger.Infof("loading media: %s", msUrl)
				client := http.Client{
					Timeout: 3600 * time.Second,
				}
				resp, err := client.Get(msUrl)
				if err != nil {
					return emperror.Wrapf(err, "cannot load url %s", msUrl)
				}
				defer resp.Body.Close()
				if resp.StatusCode >= 300 {
					logger.Errorf("cannot get image: %v - %s", resp.StatusCode, resp.Status)
					//return errors.New(fmt.Sprintf("cannot get image: %v - %s", resp.StatusCode, resp.Status))
					return nil
				}
				img, _, err := image.Decode(resp.Body)
				if err != nil {
					logger.Errorf("cannot decode image %s/%s/%s: %v", collection, signature, function, err)
					return nil
				}
				dst := image.NewRGBA(image.Rect(0, 0, (cHeight*img.Bounds().Max.X)/img.Bounds().Max.Y, cHeight))
				draw.ApproxBiLinear.Scale(dst, dst.Rect, img, img.Bounds(), draw.Over, nil)
				images = append(images, struct {
					url string
					img image.Image
				}{url: fmt.Sprintf("https://mediathek.hgk.fhnw.ch/amp/detail/%s", data.Signature), img: dst})
				width += int64(img.Bounds().Dx())
			}
		}
		//		logger.Debug(data)
		return nil
	}); err != nil {
		logger.Panic(err)
	}
	rand.Seed(time.Now().UnixNano())
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
		key := i
		img := images[key]
		//	for key, img := range images {
		logger.Infof("collage image #%v of %v", key, len(images))
		draw.Copy(coll,
			image.Point{X: posX, Y: row * cHeight},
			img.img,
			img.img.Bounds(),
			draw.Over,
			nil)
		if _, ok := positions[img.url]; !ok {
			positions[img.url] = []image.Rectangle{}
		}
		positions[img.url] = append(positions[img.url], image.Rectangle{
			Min: image.Point{X: posX, Y: row * cHeight},
			Max: image.Point{X: posX + img.img.Bounds().Dx(), Y: row*cHeight + img.img.Bounds().Dy()},
		})
		posX += img.img.Bounds().Max.X
		if posX > intDx {
			posX = 0
			row++
			// repeat cropped image
			i--
		}
		if (row+1)*cHeight > intDy {
			logger.Infof("collage %v images of %v", key, len(images))
			break
		}
	}
	fp, err := os.Create(filepath.Join(config.ExportPath, "collage.png"))
	if err != nil {
		emperror.Panic(emperror.Wrap(err, "cannot create collage file"))
	}
	if err := png.Encode(fp, coll); err != nil {
		fp.Close()
		emperror.Panic(emperror.Wrap(err, "cannot encode collage png"))
	}
	fp.Close()

	fp, err = os.Create(filepath.Join(config.ExportPath, "collage.json"))
	if err != nil {
		emperror.Panic(emperror.Wrap(err, "cannot create collage json file"))
	}
	jsonW := json.NewEncoder(fp)
	if err := jsonW.Encode(positions); err != nil {
		fp.Close()
		emperror.Panic(emperror.Wrap(err, "cannot store json"))
	}
	fp.Close()

}
