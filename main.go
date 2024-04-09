package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"read-googlesheet-go/model"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

// in code implement read file in google sheet and notification to line noti check firewall expires
func main() {
	credentialsFile := "" // service account in google cloud
	srv, err := sheets.NewService(context.Background(), option.WithCredentialsFile(credentialsFile))
	if err != nil {
		log.Fatalf("Unable to retrieve Sheets client: %v", err)
	}

	spreadsheetID := model.SheetID
	sheetTitle := model.PageTitle
	readRange := sheetTitle + "!" + model.RangeSheet

	resp, err := srv.Spreadsheets.Values.Get(spreadsheetID, readRange).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from sheet: %v", err)
	}

	var expiredServices []string
	if len(resp.Values) == 0 {
		fmt.Println("No data found.")
	} else {
		for _, row := range resp.Values[1:] {

			if len(row) >= 3 {
				expireDateStr := row[0].(string)
				source := row[1].(string)
				env := row[2].(string)
				expireDate, err := time.Parse("2006-01-02", expireDateStr)
				if err != nil {
					fmt.Printf("Error parsing date: %v\n", err)
					continue
				}

				expireDate = expireDate.Truncate(24 * time.Hour)
				timeDifference := expireDate.Sub(time.Now().Truncate(24 * time.Hour))
				if timeDifference <= 30*24*time.Hour {
					message := fmt.Sprintf("- Exp: %s,\n- Name: %s,\n- Env: %s\n", expireDate.Format("2006-01-02"), source, env)
					expiredServices = append(expiredServices, message)
					fmt.Printf("Sending notification for service expiring in less than 30 days: %v\n", source)
				}
			} else {
				fmt.Println("Invalid row data.")
			}
		}
	}
	if len(expiredServices) > 0 {
		notificationMessage := fmt.Sprintf("\n%s%s", model.LineMessage, strings.Join(expiredServices, "\n"))
		sendLineNoti(model.LineAccessToken, notificationMessage, model.StickerPackageID, model.StickerID)
	} else {
		sendLineNoti(model.LineAccessToken, "\nFire Wall Not Expires", model.StickerPackageSuccess, model.StickerSuccess)
	}

}

// send line noti
func sendLineNoti(accessToken, message string, stickerPackageId, stickerId int) (err error) {
	body := strings.NewReader(url.Values{
		"message":          []string{message},
		"stickerPackageId": []string{strconv.Itoa(stickerPackageId)},
		"stickerId":        []string{strconv.Itoa(stickerId)},
	}.Encode())
	contentType := "application/x-www-form-urlencoded"

	err = sendToLineServer(body, accessToken, contentType)
	return
}

// connect line noti
func sendToLineServer(body io.Reader, accessToken, contentType string) (err error) {
	req, err := http.NewRequest("POST", "https://notify-api.line.me/api/notify", body)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	res, err := client.Do(req)
	if err != nil {
		return
	}

	var responseBody struct {
		Status  int    `json:"status"`
		Message string `json:"message"`
	}

	if err = json.NewDecoder(res.Body).Decode(&responseBody); err != nil {
		return
	}
	defer res.Body.Close()

	if responseBody.Status != 200 {
		err = fmt.Errorf("%d: %s", responseBody.Status, responseBody.Message)
	}
	return
}
