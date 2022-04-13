package main

import (
	"context"
	"fmt"
	"google.golang.org/api/customsearch/v1"
	"google.golang.org/api/option"
	"log"
)

const (
	apiKey = "AIzaSyBfYHNyyspnojxDzqhpRuAMy5bQQ-iNwDE"
	cx     = "013219768911029609903:u7pzivy74dt"
	query  = "gestaltung"
)

func main() {

	svc, err := customsearch.NewService(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatal(err)
	}

	resp, err := svc.Cse.List().Q(query).Cx(cx).Num(10).Start(10).Do()
	if err != nil {
		log.Fatal(err)
	}

	for i, result := range resp.Items {
		fmt.Printf("#%d: %s\n", i+1, result.Title)
		fmt.Printf("\t%s\n", result.Snippet)
	}
}
