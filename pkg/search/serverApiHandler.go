package search

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"sync"
	"time"
)

type ApiResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Result  interface{} `json:"result,omitempty"`
}

func (s *Server) apiHandlerPing(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	j := json.NewEncoder(w)
	if err := j.Encode(ApiResult{
		Status:  "ok",
		Message: "service available",
		Result:  nil,
	}); err != nil {
		s.log.Errorf("cannot return error message: %v", err)
	}
}

func (s *Server) apiHandlerSignatureCreate(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	var data = &SourceData{}

	/*
		decoder := json.NewDecoder(req.Body)
		err := decoder.Decode(&data)
	*/
	defer req.Body.Close()
	bdata, err := ioutil.ReadAll(req.Body)
	if err != nil {
		msg := fmt.Sprintf("cannot read request body: %v", err)
		s.log.Errorf(msg)
		j := json.NewEncoder(w)
		w.WriteHeader(http.StatusBadRequest)
		if err := j.Encode(ApiResult{
			Status:  "error",
			Message: msg,
			Result:  nil,
		}); err != nil {
			s.log.Errorf("cannot return error message: %v", err)
		}
		return
	}

	if err := json.Unmarshal(bdata, data); err != nil {
		msg := fmt.Sprintf("cannot unmarshal request body [%s]: %v", string(bdata), err)
		s.log.Errorf(msg)
		j := json.NewEncoder(w)
		w.WriteHeader(http.StatusBadRequest)
		if err := j.Encode(ApiResult{
			Status:  "error",
			Message: msg,
			Result:  nil,
		}); err != nil {
			s.log.Errorf("cannot return error message: %v", err)
		}
		return
	}
	if err := s.mts.se.UpdateTimestamp(data, time.Now()); err != nil {
		msg := fmt.Sprintf("cannot update item: %v", err)
		s.log.Errorf(msg)
		j := json.NewEncoder(w)
		w.WriteHeader(http.StatusInternalServerError)
		if err := j.Encode(ApiResult{
			Status:  "error",
			Message: msg,
			Result:  nil,
		}); err != nil {
			s.log.Errorf("cannot return error message: %v", err)
		}
		return
	}
	j := json.NewEncoder(w)
	w.WriteHeader(http.StatusCreated)
	if err := j.Encode(ApiResult{
		Status:  "ok",
		Message: fmt.Sprintf("item %s created", data.Signature),
		Result:  nil,
	}); err != nil {
		s.log.Errorf("cannot return error message: %v", err)
	}

}

func (s *Server) apiHandlerSignaturesDelete(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	vars := mux.Vars(req)

	prefix, ok := vars["prefix"]
	if !ok {
		msg := "no prefix for signature deletion found"
		s.log.Infof("error in apiHandlerSignaturesDelete: %s", msg)
		j := json.NewEncoder(w)
		w.WriteHeader(http.StatusBadRequest)
		if err := j.Encode(ApiResult{
			Status:  "error",
			Message: msg,
			Result:  nil,
		}); err != nil {
			s.log.Errorf("cannot return error message: %v", err)
		}
		return
	}

	cfg := &ScrollConfig{
		FiltersFields: map[string][]string{
			"signature": []string{fmt.Sprintf("%s*", prefix)},
		},
		QStr:           "",
		Groups:         []string{},
		ContentVisible: false,
		IsAdmin:        true,
	}
	num, err := s.mts.se.Delete(cfg)
	if err != nil {
		msg := fmt.Sprintf("error deleting signatures %s: %v", prefix, err)
		s.log.Infof("error in apiHandlerSignaturesDelete: %s", msg)
		j := json.NewEncoder(w)
		w.WriteHeader(http.StatusInternalServerError)
		if err := j.Encode(ApiResult{
			Status:  "error",
			Message: msg,
			Result:  nil,
		}); err != nil {
			s.log.Errorf("cannot return error message: %v", err)
		}
		return
	}
	msg := fmt.Sprintf("%v signatures with prefix %s deleted", num, prefix)
	s.log.Infof("apiHandlerSignaturesDelete: %s", msg)
	j := json.NewEncoder(w)
	if err := j.Encode(ApiResult{
		Status:  "ok",
		Message: msg,
		Result:  num,
	}); err != nil {
		s.log.Errorf("cannot return error message: %v", err)
	}
}

func (s *Server) apiHandlerLastUpdate(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	vars := mux.Vars(req)

	prefix, ok := vars["prefix"]
	if !ok {
		msg := "no prefix for signature deletion found"
		s.log.Infof("error in apiHandlerSignaturesDelete: %s", msg)
		j := json.NewEncoder(w)
		w.WriteHeader(http.StatusBadRequest)
		if err := j.Encode(ApiResult{
			Status:  "error",
			Message: msg,
			Result:  nil,
		}); err != nil {
			s.log.Errorf("cannot return error message: %v", err)
		}
		return
	}

	cfg := &ScrollConfig{
		FiltersFields: map[string][]string{
			"signature": []string{fmt.Sprintf("%s*", prefix)},
		},
		QStr:           "",
		Groups:         []string{},
		ContentVisible: false,
		IsAdmin:        true,
	}
	last, err := s.mts.se.LastUpdate(cfg)
	if err != nil {
		msg := fmt.Sprintf("error getting last update %s: %v", prefix, err)
		s.log.Infof("error in apiHandlerLastUpdate: %s", msg)
		j := json.NewEncoder(w)
		w.WriteHeader(http.StatusInternalServerError)
		if err := j.Encode(ApiResult{
			Status:  "error",
			Message: msg,
			Result:  nil,
		}); err != nil {
			s.log.Errorf("cannot return error message: %v", err)
		}
		return
	}
	msg := fmt.Sprintf("last update of %s at %v", prefix, last)
	s.log.Infof("apiHandlerLastUpdate: %s", msg)
	j := json.NewEncoder(w)
	if err := j.Encode(ApiResult{
		Status:  "ok",
		Message: msg,
		Result:  last,
	}); err != nil {
		s.log.Errorf("cannot return error message: %v", err)
	}
}

func (s *Server) apiHandlerClearCache(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	if err := s.mts.clearCache(); err != nil {
		msg := fmt.Sprintf("cannot clear cache: %v", err)
		s.log.Errorf(msg)
		j := json.NewEncoder(w)
		w.WriteHeader(http.StatusInternalServerError)
		if err := j.Encode(ApiResult{
			Status:  "error",
			Message: msg,
			Result:  nil,
		}); err != nil {
			s.log.Errorf("cannot return error message: %v", err)
		}
		return
	}
	j := json.NewEncoder(w)
	w.WriteHeader(http.StatusOK)
	if err := j.Encode(ApiResult{
		Status:  "ok",
		Message: "cache cleared",
		Result:  nil,
	}); err != nil {
		s.log.Errorf("cannot return error message: %v", err)
	}

}

var sitemapMutex sync.Mutex

func (s *Server) apiHandlerBuildSitemap(w http.ResponseWriter, req *http.Request) {
	sitemapMutex.Lock()
	defer sitemapMutex.Unlock()
	if err := s.buildSitemap(); err != nil {
		msg := "error building sitemap"
		s.log.Infof("error in apiHandlerBuildSitemap: %s", msg)
		j := json.NewEncoder(w)
		w.WriteHeader(http.StatusInternalServerError)
		if err := j.Encode(ApiResult{
			Status:  "error",
			Message: msg,
			Result:  nil,
		}); err != nil {
			s.log.Errorf("cannot return error message: %v", err)
		}
		return
	}

	msg := "build sitemap done"
	s.log.Infof("apiHandlerBuildSitemap: %s", msg)
	j := json.NewEncoder(w)
	if err := j.Encode(ApiResult{
		Status:  "ok",
		Message: msg,
		Result:  nil,
	}); err != nil {
		s.log.Errorf("cannot return error message: %v", err)
	}
}
