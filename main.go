package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/debug"
)

type User struct {
	ProfileID        string `json:"Profile Id"`
	FullName         string `json:"Full Name"`
	ProfileLink      string `json:"ProfileLink"`
	Bio              string `json:"Bio"`
	ImageSrc         string `json:"Image Src"`
	GroupID          string `json:"Group Id"`
	GroupJoiningText string `json:"Group Joining Text"`
	Type             string `json:"ProfileType"`
	ProfilePhotoURL  string
	Avatar           string
}

type PageInfo struct {
	HasNextPage bool   `json:"has_next_page"`
	EndCursor   string `json:"end_cursor"`
}

type GraphResponse struct {
	Data struct {
		Node struct {
			GroupID string `json:"id"`
			Members struct {
				Edges []struct {
					Node struct {
						ID  string `json:"id"`
						URL string `json:"url"`
					} `json:"node"`
				} `json:"edges"`
				PageInfo PageInfo `json:"page_info"`
			} `json:"new_members"`
		} `json:"node"`
	} `json:"data"`
}

var nativeCookies []*http.Cookie
var headerVariablesString string
var err error

func main() {

	var facebookGroupID = os.Args[1]

	fmt.Println("Get Page info")

	nativeCookies = BuildCookies()

	headerVariablesContent, err := os.Open("./variables.txt")

	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}

	defer headerVariablesContent.Close()

	byteValue, _ := ioutil.ReadAll(headerVariablesContent)
	headerVariablesString = string(byteValue)

	c := colly.NewCollector(colly.Debugger(&debug.LogDebugger{}))

	c.SetCookies("https://www.facebook.com", nativeCookies)

	c.OnRequest(func(r *colly.Request) {
		r = SetHeaderRequest(r)
	})

	c.OnError(func(r *colly.Response, e error) {
		log.Fatalln(e)
	})

	c.OnHTML("html", func(e *colly.HTMLElement) {
		hasNextPageRegular := regexp.MustCompile(`{("has_next_page.*?)}`)
		hasNextPageMatch := hasNextPageRegular.FindAllStringSubmatch(e.Text, 10)

		if len(hasNextPageMatch) == 0 {
			log.Fatalln("Page info not found")
		}

		var pageInfo PageInfo

		err = json.Unmarshal([]byte(hasNextPageMatch[0][0]), &pageInfo)

		if err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("Start page info %v \n", pageInfo)
		fmt.Println("Get Page info success. Start crawler...")

		FetchNextpage(facebookGroupID, pageInfo)

	})

	c.Visit(fmt.Sprintf("https://www.facebook.com/groups/%s/members", facebookGroupID))

	fmt.Println("Complate")

}

func FetchNextpage(facebookGroupID string, pageInfo PageInfo) {

	// fmt.Println("Next page" + pageInfo.EndCursor)

	url := "https://www.facebook.com/api/graphql/"
	method := "POST"

	cursorRegexp := regexp.MustCompile(`"(cursor.*?),`)
	result := cursorRegexp.ReplaceAllString(headerVariablesString, `"cursor":"`+pageInfo.EndCursor+`",`)

	groupRegexp := regexp.MustCompile(`"(groupID.*?),`)
	result = groupRegexp.ReplaceAllString(result, `"groupID":"`+facebookGroupID+`",`)

	idRegexp := regexp.MustCompile(`"(id.*?)}`)
	result = idRegexp.ReplaceAllString(result, `"id":"`+facebookGroupID+`"}`)

	payload := strings.NewReader(result)

	client := &http.Client{}
	req, err := http.NewRequest(method, url, payload)

	if err != nil {
		log.Fatalln(err)
	}

	req.Header.Add("authority", "www.facebook.com")
	req.Header.Add("accept", "*/*")
	req.Header.Add("accept-language", "en-US,en;q=0.7")
	req.Header.Add("cache-control", "no-cache")
	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	req.Header.Add("origin", "https://www.facebook.com")
	req.Header.Add("pragma", "no-cache")
	req.Header.Add("referer", "https://www.facebook.com/groups/"+facebookGroupID+"/members")
	req.Header.Add("sec-ch-ua", "\"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"108\", \"Brave\";v=\"108\"")
	req.Header.Add("sec-ch-ua-mobile", "?0")
	req.Header.Add("sec-ch-ua-platform", "\"macOS\"")
	req.Header.Add("sec-fetch-dest", "empty")
	req.Header.Add("sec-fetch-mode", "cors")
	req.Header.Add("sec-fetch-site", "same-origin")
	req.Header.Add("sec-gpc", "1")
	req.Header.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36")
	req.Header.Add("x-fb-friendly-name", "GroupsCometMembersPageNewMembersSectionRefetchQuery")
	req.Header.Add("x-fb-lsd", "1HapSgzxymrq1sXUyYilCG")

	var c string
	for _, v := range nativeCookies {
		c = c + v.Name + "=" + v.Value + "; "
	}

	req.Header.Add("cookie", c)

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return
	}

	var response GraphResponse

	err = json.Unmarshal(body, &response)

	if err != nil {
		log.Fatalln(err)
	}

	for _, v := range response.Data.Node.Members.Edges {
		var user User
		user.ProfileID = v.Node.ID
		user.ProfileLink = v.Node.URL
		GetProfileAvatarLink(user)
	}

	fmt.Printf("Next page info %v", response.Data.Node.Members.PageInfo)

	if response.Data.Node.Members.PageInfo.HasNextPage {
		FetchNextpage(facebookGroupID, response.Data.Node.Members.PageInfo)
	}

}

func BuildCookies() []*http.Cookie {

	type Cookie struct {
		Domain   string `json:"domain"`
		HostOnly bool   `json:"hostOnly"`
		HttpOnly bool   `json:"httpOnly"`
		Name     string `json:"name"`
		Path     string `json:"path"`
		SameSite string `json:"sameSite"`
		Secure   bool   `json:"secure"`
		Session  bool   `json:"session"`
		StoreId  string `json:"storeId"`
		Value    string `json:"value"`
	}

	jsonFile, err := os.Open("./cookies.json")

	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}

	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)

	json.Unmarshal(byteValue, &nativeCookies)

	return nativeCookies

}

func FetchUserAvatar(user User) {

	fmt.Println(user.ProfilePhotoURL)

	c := colly.NewCollector()
	c.SetCookies("https://www.facebook.com", nativeCookies)

	c.OnHTML("script", func(e *colly.HTMLElement) {

		matchScriptHasImage, _ := regexp.MatchString(`"result":\{"data":\{"currMedia":\{"__typename":"Photo","__isMedia":"Photo"`, e.Text)

		if matchScriptHasImage {
			matchImageRegexp := regexp.MustCompile(`"(https.*?)"`)
			images := matchImageRegexp.FindStringSubmatch(e.Text)
			if len(images) < 0 {
				fmt.Println("Image not found")
			} else {
				avatar := strings.ReplaceAll(images[0], "\\/", "/")
				user.Avatar = strings.Trim(avatar, "\"")

				writeData := [][]string{
					{
						user.ProfileLink,
						user.Avatar,
					},
				}

				file, err := os.OpenFile("./members.csv", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					log.Fatal(err)
				}

				if err != nil {
					log.Fatal(err)
				}
				defer file.Close()

				if err != nil {
					log.Fatal(err)
				}

				w := csv.NewWriter(file)
				w.WriteAll(writeData)
			}
		}

	})

	c.OnRequest(func(r *colly.Request) {
		r = SetHeaderRequest(r)
	})

	c.Visit(user.ProfilePhotoURL)

}

func GetProfileAvatarLink(user User) {

	c := colly.NewCollector()
	c.SetCookies("https://www.facebook.com", nativeCookies)

	c.OnRequest(func(r *colly.Request) {

		r = SetHeaderRequest(r)
		fmt.Println(user.ProfileLink)
	})

	c.OnHTML("html", func(e *colly.HTMLElement) {
		profileAvatarURLRegexp := regexp.MustCompile(`"(https:\\\/\\\/www\.facebook\.com\\\/photo\\\/\?fbid.*?)"`)
		profilePhotoURLs := profileAvatarURLRegexp.FindAllStringSubmatch(e.Text, 10)
		if len(profilePhotoURLs) < 2 {
			fmt.Println("Profile url not found")
		} else {
			profileURLWithDoubleQuote := strings.ReplaceAll(profilePhotoURLs[1][1], "\\/", "/")
			profileURL := strings.ReplaceAll(profileURLWithDoubleQuote, `"`, ``)
			user.ProfilePhotoURL = profileURL

			FetchUserAvatar(user)
		}

	})

	c.Visit(user.ProfileLink)

}

func SetHeaderRequest(r *colly.Request) *colly.Request {
	r.Headers.Add("authority", "www.facebook.com")
	r.Headers.Add("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	r.Headers.Add("accept-language", "en-US,en;q=0.7")
	r.Headers.Add("cache-control", "no-cache")
	r.Headers.Add("sec-ch-ua", "\"Not?A_Brand\";v=\"8\", \"Chromium\";v=\"108\", \"Brave\";v=\"108\"")
	r.Headers.Add("sec-ch-ua-mobile", "?0")
	r.Headers.Add("sec-ch-ua-platform", "\"macOS\"")
	r.Headers.Add("sec-fetch-dest", "document")
	r.Headers.Add("sec-fetch-mode", "navigate")
	r.Headers.Add("sec-fetch-site", "same-origin")
	r.Headers.Add("sec-fetch-user", "?1")
	r.Headers.Add("sec-gpc", "1")
	r.Headers.Add("upgrade-insecure-requests", "1")
	r.Headers.Add("user-agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36")

	return r
}
