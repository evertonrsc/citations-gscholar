package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	gs "github.com/serpapi/google-search-results-golang"
)

var (
	serpapi_key string
	paperTitle  string
	inputFile   string
	outCsvFile  string
)

type PublicationInfo struct {
	resultId  string
	title     string
	year      int
	authors   []string
	citations int
	citesId   string
}

func init() {
	keyfile, err := os.Open("serpapi.key")
	checkError(err)
	defer keyfile.Close()

	reader := bufio.NewReader(keyfile)
	serpapi_key, err = reader.ReadString('\n')
	if err == io.EOF {
		return
	}
}

func readPapers(filename string) ([]string, error) {
	inputFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer inputFile.Close()

	var papers []string
	scanner := bufio.NewScanner(inputFile)
	for scanner.Scan() {
		papers = append(papers, scanner.Text())
	}

	return papers, scanner.Err()
}

func getPublicationInfo(paperTitle string) PublicationInfo {
	gsparameters := map[string]string{
		"engine":  "google_scholar",
		"q":       fmt.Sprintf("%q", paperTitle),
		"api_key": serpapi_key,
		"hl":      "en",
	}
	search := gs.NewGoogleSearch(gsparameters, gsparameters["api_key"])
	response, err := search.GetJSON()
	checkError(err)

	organicResults := response["organic_results"].([]interface{})
	content := organicResults[0].(map[string]interface{})

	var publication PublicationInfo
	publication.title = paperTitle
	publication.resultId = content["result_id"].(string)

	summary := content["publication_info"].(map[string]interface{})["summary"].(string)
	lastIndex := strings.LastIndex(summary, " -")
	publication.year, _ = strconv.Atoi(summary[lastIndex-4 : lastIndex])

	citedBy := content["inline_links"].(map[string]interface{})["cited_by"]
	if citedBy != nil {
		publication.citations = int(citedBy.(map[string]interface{})["total"].(float64))
		publication.citesId = citedBy.(map[string]interface{})["cites_id"].(string)
	}

	gscparameters := map[string]string{
		"engine":  "google_scholar_cite",
		"q":       publication.resultId,
		"api_key": serpapi_key,
		"hl":      "en",
	}
	search = gs.NewGoogleSearch(gscparameters, gsparameters["api_key"])
	response, err = search.GetJSON()
	checkError(err)

	citations := response["citations"].([]interface{})
	vancouverCite := citations[len(citations)-1].(map[string]interface{})
	citeVancouverSnippet := vancouverCite["snippet"].(string)
	publication.authors = strings.Split(citeVancouverSnippet[:strings.Index(citeVancouverSnippet, ".")], ", ")
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
			"hl":      "en",
		}
		search := gs.NewGoogleSearch(gsparameters, gsparameters["api_key"])
		response, err := search.GetJSON()
		checkError(err)

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

func getAverageCitations(paperTitle string) float32 {
	currentYear := time.Now().Year()
	publicationInfo := getPublicationInfo(paperTitle)
	return float32(publicationInfo.citations) / float32(currentYear-publicationInfo.year)
}

func printCitationCounts(paperTitle string, csvContents *string) {
	totalCitations := getTotalCitations(paperTitle)
	averageCitations := getAverageCitations(paperTitle)
	organicCitations := getOrganicCitations(paperTitle)
	fmt.Printf("  Total: %d, Average: %.1f, Organic: %d\n\n", totalCitations, averageCitations, organicCitations)
	*csvContents += fmt.Sprintf("\"%s\",%d,%.2f,%d\n", paperTitle, totalCitations, averageCitations, organicCitations)
}

func flushCsv(outCsvFile, csvContents string) {
	csvfile, err := os.Create(outCsvFile)
	checkError(err)
	csvfile.WriteString(csvContents)
	csvfile.Close()
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	flag.StringVar(&paperTitle, "p", "", "Paper title for obtaining citation counts")
	flag.StringVar(&inputFile, "f", "", "Text file with a list of paper titles for obtaining citation counts")
	flag.StringVar(&outCsvFile, "o", "", "CSV file to output citation counts")
	flag.Parse()

	if len(os.Args) == 1 {
		fmt.Println("Error: using at least one flag is mandatory")
		fmt.Println("Usage of citations-gscholar:")
		fmt.Println("  -p string\n\tPaper title for obtaining citation counts")
		fmt.Println("  -f string\n\tText file with a list of paper titles for obtaining citation counts")
		fmt.Println("  -o string\n\tCSV file to output citation counts")
		os.Exit(1)
	}

	fmt.Println(serpapi_key)
	os.Exit(0)

	csvContents := "Title,Total,Average,Organic\n"

	if paperTitle != "" {
		fmt.Printf("> Obtaining citation counts for \"%s\"\n", paperTitle)
		printCitationCounts(paperTitle, &csvContents)
	}

	if inputFile != "" {
		papers, err := readPapers(inputFile)
		checkError(err)
		for _, paperTitle := range papers {
			fmt.Printf("> Obtaining citation counts for \"%s\"\n", paperTitle)
			printCitationCounts(paperTitle, &csvContents)
		}
	}

	if outCsvFile != "" {
		flushCsv(outCsvFile, csvContents)
	}
}
