package main

import (
    "fmt"
    "html/template"
    "log"
	"net/http"
	"net/url"
	"strings"
	"hello-backend/test"	
)

func sayhelloName(w http.ResponseWriter, r *http.Request) {
    r.ParseForm() //Parse url parameters passed, then parse the response packet for the POST body (request body)
    // attention: If you do not call ParseForm method, the following data can not be obtained form
    fmt.Println(r.Form) // print information on server side.
    fmt.Println("path", r.URL.Path)
    fmt.Println("scheme", r.URL.Scheme)
    fmt.Println(r.Form["url_long"])
    for k, v := range r.Form {
        fmt.Println("key:", k)
        fmt.Println("val:", strings.Join(v, ""))
    }
    fmt.Fprintf(w, "Bismillah... Hello GO WEB!") // write data to response
}

func checkURL(w http.ResponseWriter, r *http.Request) {
    fmt.Println("method:", r.Method) //get request method
    if r.Method == "GET" {
        t, _ := template.ParseFiles("checkurl.gtpl")
        t.Execute(w, nil)
    } else {
        r.ParseForm()
		// logic part of check URL
		url_input := r.Form["url_input"][0]
		fmt.Println("url_input:", url_input)   
		if isValidUrl(url_input) {
			constructCheckup(w, url_input)
		} else {
			fmt.Fprintf(w, "Please Input a valid URL other than: %v\n", url_input) 
		}
		    		
    }
}

func constructCheckup(w http.ResponseWriter, url_input string) {
	
	hc := test.HTTPChecker{Name: "Test", URL: url_input, Attempts: 5}
	hc.ThresholdRTT = 0
	result, err := hc.Check()	
	if err != nil {
		fmt.Println("Error checking: %v", err)
	}
	fmt.Fprintf(w, "URL input: %v\n", url_input) 
	fmt.Fprintf(w, "\nResults: %v\n", result) 
}

func isValidUrl(str string) bool {
    u, err := url.Parse(str)
    return err == nil && u.Scheme != "" && u.Host != ""
}

func main() {
	fmt.Println("Bismillah.. Hello GO")
	
	http.HandleFunc("/", sayhelloName)
	http.HandleFunc("/checkurl", checkURL)
	err := http.ListenAndServe(":9090", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}