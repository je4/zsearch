package search

import (
	"fmt"
	"github.com/je4/sitemap/v2"
	"github.com/pkg/errors"
	"os"
	"time"
)

func (s *Server) buildSitemap() error {
	var size int64 = 3000
	cfg := &ScrollConfig{
		FiltersFields:  map[string][]string{"catalog": s.baseCatalog},
		QStr:           "",
		Groups:         []string{"global/guest"},
		ContentVisible: true,
		IsAdmin:        false,
	}

	var counter int64 = 0
	var sitemapNo int64 = 0
	var sitemapindex *sitemap.SitemapIndex = sitemap.NewSitemapIndex()
	var sm *sitemap.Sitemap = sitemap.New()

	var sitemapPrefix = "zsearch"

	if err := s.mts.se.Scroll(cfg, func(data *SourceData) error {
		//		log.Infof("%0.5d - %v", counter, data.Signature)
		if counter%size == 0 {
			if counter > 0 {
				filename := fmt.Sprintf("%s/%s-%05d.xml", s.sitemapDir, sitemapPrefix, sitemapNo)
				file, err := os.OpenFile(filename, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					return errors.Wrapf(err, "cannot create file %s", filename)
				}
				if _, err := sm.WriteTo(file); err != nil {
					s.log.Errorf("buildSitemap: cannot write xml to %s: %v", filename, err)
				}
				file.Close()
				s.log.Infof("buildSitemap: %v written", filename)

				lastMod := time.Now()
				u := &sitemap.URL{
					Loc:     fmt.Sprintf("%s/%s/%s-%05d.xml", s.addrExt, s.prefixes["sitemap"], sitemapPrefix, sitemapNo),
					LastMod: &lastMod,
				}
				sitemapindex.Add(u)
				sitemapNo++
				sm = sitemap.New()
			}
		}
		us := fmt.Sprintf("%s/%s/%s", s.addrExt, s.prefixes["detail"], data.Signature)
		u := &sitemap.URL{
			Loc:     us,
			LastMod: &data.Timestamp,
		}
		sm.Add(u)
		counter++
		return nil
	}); err != nil {
		return err
	}
	if len(sm.URLs) > 0 {
		filename := fmt.Sprintf("%s/%s-%05d.xml", s.sitemapDir, sitemapPrefix, sitemapNo)
		file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return errors.Wrapf(err, "cannot create file %s", filename)
		}
		if _, err := sm.WriteTo(file); err != nil {
			s.log.Errorf("buildSitemap: cannot write xml to %s: %v", filename, err)
		}
		file.Close()
		s.log.Infof("buildSitemap: %v written", filename)
		lastMod := time.Now()
		u := &sitemap.URL{
			Loc:     fmt.Sprintf("%s/%s/%s-%05d.xml", s.addrExt, s.prefixes["sitemap"], sitemapPrefix, sitemapNo),
			LastMod: &lastMod,
		}
		sitemapindex.Add(u)
	}

	filename := fmt.Sprintf("%s/%s.xml", s.sitemapDir, sitemapPrefix)
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrapf(err, "cannot create file %s", filename)
	}
	sitemapindex.WriteTo(file)
	file.Close()
	s.log.Infof("buildSitemap: %v written", filename)

	return nil
}
