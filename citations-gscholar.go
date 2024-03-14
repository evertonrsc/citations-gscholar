package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	gs "github.com/serpapi/google-search-results-golang"
)

const (
	serpapi_key = "e41d3f15bd08b41ade61bbac3c14fbb87f1b658d01f699d8df1d0a5583f6f6c0"
)

type PublicationInfo struct {
	resultId  string
	title     string
	authors   []string
	citations int
	citesId   string
}

func readPapers(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var papers []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		papers = append(papers, scanner.Text())
	}

	return papers, scanner.Err()
}

func getPublicationInfo(paperTitle string) PublicationInfo {
	gsparameters := map[string]string{
		"engine":  "google_scholar",
		"q":       paperTitle,
		"api_key": serpapi_key,
	}
	search := gs.NewGoogleSearch(gsparameters, gsparameters["api_key"])
	response, err := search.GetJSON()
	if err != nil {
		log.Fatal(err)
	}

	organicResults := response["organic_results"].([]interface{})
	content := organicResults[0].(map[string]interface{})

	var publication PublicationInfo
	publication.title = paperTitle
	publication.resultId = content["result_id"].(string)
	citedBy := content["inline_links"].(map[string]interface{})["cited_by"]
	if citedBy != nil {
		publication.citations = int(citedBy.(map[string]interface{})["total"].(float64))
		publication.citesId = citedBy.(map[string]interface{})["cites_id"].(string)
	}

	gscparameters := map[string]string{
		"engine":  "google_scholar_cite",
		"q":       publication.resultId,
		"api_key": serpapi_key,
	}
	search = gs.NewGoogleSearch(gscparameters, gsparameters["api_key"])
	response, err = search.GetJSON()
	if err != nil {
		log.Fatal(err)
	}

	citations := response["citations"].([]interface{})
	vancouverCite := citations[len(citations)-1].(map[string]interface{})
	citeSnippet := vancouverCite["snippet"].(string)
	publication.authors = strings.Split(citeSnippet[:strings.Index(citeSnippet, ".")], ", ")
	return publication
}

func getTotalCitations(paperTitle string) int {
	return getPublicationInfo(paperTitle).citations
}

func getOrganicCitations(paperTitle string) int {
	var selfcite int
	publication := getPublicationInfo(paperTitle)

	if publication.citesId != "" {
		gsparameters := map[string]string{
			"engine":  "google_scholar",
			"cites":   publication.citesId,
			"api_key": serpapi_key,
		}
		search := gs.NewGoogleSearch(gsparameters, gsparameters["api_key"])
		response, err := search.GetJSON()
		if err != nil {
			log.Fatal(err)
		}

		organicResults := response["organic_results"].([]interface{})
		for i := range organicResults {
			title := organicResults[i].(map[string]interface{})["title"].(string)
			citingpub := getPublicationInfo(title)

			for _, author := range publication.authors {
				if slices.Contains(citingpub.authors, author) {
					selfcite++
					break
				}
			}
		}
	}

	return publication.citations - selfcite
}

func getCitatons(paperTitle string, startYear int) (int, float32, int) {
	totalCitations := getTotalCitations(paperTitle)
	averageCitations := float32(totalCitations) / float32(time.Now().Year()-startYear)
	organicCitations := getOrganicCitations(paperTitle)
	return totalCitations, averageCitations, organicCitations
}

func main() {
	papers, err := readPapers("papers.txt")
	if err != nil {
		log.Fatal(err)
	}

	for _, paperTitle := range papers {
		start := time.Now()
		fmt.Printf("> Obtaining citation count for \"%s\"\n", paperTitle)
		totalCitations, averageCitations, organicCitations := getCitatons(paperTitle, 2013)
		fmt.Printf("  Total: %d, Average: %.1f, Organic: %d\n", totalCitations, averageCitations, organicCitations)
		fmt.Printf("  Elapsed time %.3fs\n\n", time.Since(start).Seconds())
	}
}
