// Licensed to Elasticsearch B.V. under one or more agreements.
// Elasticsearch B.V. licenses this file to you under the Apache 2.0 License.
// See the LICENSE file in the project root for more information.

// This example demonstrates a basic usage of the Elasticsearch Go client.
//
// It fetches the version information from the cluster, indexes a couple
// of documents concurrently, and performs a search request.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"

	"github.com/elastic/go-elasticsearch/esapi"
	"github.com/elastic/go-elasticsearch/v7"
)

func main() {
	log.SetFlags(0)

	var (
		r  map[string]interface{}
		wg sync.WaitGroup
	)
	elasticConf := elasticsearch.Config{
		Addresses: []string{"http://127.0.0.1:9200"},
		Username:  "elastic",
		Password:  "1234!@#$",
	}

	// Initialize a client with the default settings.
	//
	// An `ELASTICSEARCH_URL` environment variable will be used when exported.
	//
	es, err := elasticsearch.NewClient(elasticConf)
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}

	// 1. Get cluster info
	//
	res, err := es.Info()
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
	// Check response status
	if res.IsError() {
		log.Fatalf("Error: %s", res.String())
	}

	// Deserialize the response into a map.
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		log.Fatalf("Error parsing the response body: %s", err)
	}
	// Print client and server version numbers.
	log.Printf("Client: %s", elasticsearch.Version)
	log.Printf("Server: %s", r["version"].(map[string]interface{})["number"])
	log.Println(strings.Repeat("~", 37))

	// 2. Search for the indexed documents
	//
	// Build the request body.
	var buf bytes.Buffer
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"pipeline_metadata.type": "logstash_pipeline",
			},
		},
	}
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}

	// Perform the search request.
	res, err = es.Search(
		es.Search.WithContext(context.Background()),
		es.Search.WithIndex(".logstash"),
		es.Search.WithBody(&buf),
		es.Search.WithTrackTotalHits(true),
		es.Search.WithPretty(),
	)
	if err != nil {
		log.Fatalf("Error getting response: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			log.Fatalf("Error parsing the response body: %s", err)
		} else {
			// Print the response status and error information.
			log.Fatalf("[%s] %s: %s",
				res.Status(),
				e["error"].(map[string]interface{})["type"],
				e["error"].(map[string]interface{})["reason"],
			)
		}
	}

	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		log.Fatalf("Error parsing the response body: %s", err)
	}
	// Print the response status, number of results, and request duration.
	log.Printf(
		"[%s] %d hits; took: %dms",
		res.Status(),
		int(r["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64)),
		int(r["took"].(float64)),
	)
	// Print the ID and document source for each hit.
	for _, hit := range r["hits"].(map[string]interface{})["hits"].([]interface{}) {
		source := hit.(map[string]interface{})["_source"].(map[string]interface{})
		id := fmt.Sprintf("%v", hit.(map[string]interface{})["_id"])
		log.Printf("before username=[%v]", hit.(map[string]interface{})["username1"])
		username, ok := source["username"]
		log.Printf("username_type=%s", reflect.ValueOf(username).Type())
		if reflect.ValueOf(hit.(map[string]interface{})["_id"]).String() == "main1" {
			log.Printf("username=%v, len=%v", reflect.ValueOf(username), ok)
			//if ok {
			inindex := fmt.Sprintf("%v", hit.(map[string]interface{})["_index"])
			log.Printf("index=%s %s", inindex, source)
			source["pipeline"] = `##testpipeline2
			` + fmt.Sprintf("%v", source["pipeline"])
			// 2. Index documents concurrently
			//
			wg.Add(1)

			go func(str string) {
				defer wg.Done()

				// Build the request body.
				var b strings.Builder
				output, _ := json.Marshal(source)
				log.Printf("output=%s", output)
				b.WriteString(string(output))

				// Set up the request object.
				req := esapi.IndexRequest{
					Index:      "test",
					DocumentID: id,
					Body:       strings.NewReader(b.String()),
					Refresh:    "true",
				}

				// Perform the request with the client.
				res, err := req.Do(context.Background(), es)
				if err != nil {
					log.Fatalf("Error getting response: %s", err)
				}
				defer res.Body.Close()

				if res.IsError() {
					log.Printf("[%s] Error indexing document ID=%s", res.Status(), id)
				} else {
					// Deserialize the response into a map.
					var r map[string]interface{}
					if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
						log.Printf("Error parsing the response body: %s", err)
					} else {
						// Print the response status and indexed document version.
						log.Printf("[%s] %s; version=%d", res.Status(), r["result"], int(r["_version"].(float64)))
					}
				}
			}("t")

			wg.Wait()

			log.Println(strings.Repeat("-", 37))
		}
	}

	log.Println(strings.Repeat("=", 37))

}
