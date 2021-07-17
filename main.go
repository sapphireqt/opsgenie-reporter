package main

import (
	"context"
	"fmt"
	"github.com/opsgenie/opsgenie-go-sdk-v2/alert"
	"github.com/opsgenie/opsgenie-go-sdk-v2/client"
	"github.com/sirupsen/logrus"
	"encoding/json"
	"github.com/parnurzeal/gorequest"
	//"github.com/slack-go/slack"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

var NameSpaces = [...]string {
	"ingress-nginx",
	"kube-system",
	"default",
}

type kv struct {
	Key   string
	Value int
}

func main () {
	Log := setLogger()
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	yesterdayStart:= time.Now().Truncate(24 * time.Hour).Add(-24 * time.Hour).Unix()
	yesterdayEnd := time.Now().Truncate(24 * time.Hour).Unix()
	weekAgoS := yesterdayStart - 604800
	monthAgoS := yesterdayStart - 2592000

	ys := make(map[string]int)
	wa := make(map[string]int)
	ma := make(map[string]int)
	ysTop := make(map[string]int)

	Log.Println("starting alert stats bot..")

	config := &client.Config{
		ApiKey: os.Getenv("OPSGENIE_API_KEY"),
	}

	alertClient, err := alert.NewClient(config)
	if err != nil {
		Log.Errorf("Failed to init opsgenie client: %w", err)
	}

	alertsList, err := listAlerts(ctx, alertClient, yesterdayStart, yesterdayEnd)
	if err != nil {
		Log.Errorf("Failed to list alerts %w", err)
	}

	ysTop = top5Alerts(alertsList)
	sortedysTop := sortMapByValueDesc(ysTop)

	for i := range alertsList {
		for j := range alertsList[i].Tags {
			ys[alertsList[i].Tags[j]] = ys[alertsList[i].Tags[j]] + 1
		}
	}
	payload1 := fmt.Sprintf("Вчера *%s* было *%d* критикал алертов\n\n", time.Now().AddDate(0, 0, -1).Format("2006-01-02"), len(alertsList))
	for x := range sortedysTop {
		payload1 += fmt.Sprintf("*%s* сработал *%d* %s\n", sortedysTop[x].Key, sortedysTop[x].Value, razRaza(sortedysTop[x].Value))
	}

	alertsList, err = listAlerts(ctx, alertClient, weekAgoS, yesterdayEnd)
	if err != nil {
		Log.Errorf("Failed to list alerts %w", err)
	}
	for i := range alertsList {
		for j := range alertsList[i].Tags {
			wa[alertsList[i].Tags[j]] = wa[alertsList[i].Tags[j]] + 1
		}
	}

	alertsList, err = listAlerts(ctx, alertClient, monthAgoS, yesterdayEnd)
	if err != nil {
		Log.Errorf("Failed to list alerts %w", err)
	}
	for i := range alertsList {
		for j := range alertsList[i].Tags {
			ma[alertsList[i].Tags[j]] = ma[alertsList[i].Tags[j]] + 1
		}
	}

	payload2 := "*Командный топ*:\n"
	yss := sortMapByValueDesc(ys)
	for j := range yss {
		for s := range NameSpaces{
			if yss[j].Key == NameSpaces[s] && yss[j].Value > 0 {
				payload2 += fmt.Sprintf("*%s*:\n Вчера: %2d,\f Неделя: %3d,\f Месяц: %3d\n",
					NameSpaces[s], ys[NameSpaces[s]], wa[NameSpaces[s]], ma[NameSpaces[s]])
			}
		}

	}

	err = postSlackMessage(os.Getenv("SLACK_CHANNEL"), payload1 + "\n" + payload2)
	if err != nil {
		Log.Errorf("Failed to slack: %w", err)
	}
}

func setLogger() (*logrus.Logger) {
		Logger := logrus.New()
		Logger.SetFormatter(
			&logrus.TextFormatter{
				ForceColors:     true,
				FullTimestamp:   true,
				TimestampFormat: time.RFC3339Nano,
			},
		)
		return Logger
}

func listAlerts (ctx context.Context, alertClient *alert.Client, startUnix int64, endUnix int64) ([]alert.Alert, error) {
	alertQuery := fmt.Sprintf("tag: Production and priority: P1 and createdAt > %d and createdAt < %d", startUnix, endUnix)
	var alertList []alert.Alert
	var iter int

	for {
		alertListResult, err := alertClient.List(ctx, &alert.ListAlertRequest{
			Limit:  100,
			Offset: iter*100,
			Query: alertQuery,
		})
		if err != nil {
			return nil, err
		}
		if len(alertListResult.Alerts) == 0 {
			return alertList, nil
		}
		for i := range alertListResult.Alerts {
			alertList = append(alertList, alertListResult.Alerts[i])
		}
		iter = iter + 1
		time.Sleep(100 * time.Millisecond)
	}
}

func sortMapByValueDesc (m map[string]int) []kv {
	var ss []kv
	for k, v := range m {
		ss = append(ss, kv{k, v})
	}

	sort.Slice(ss, func(i, j int) bool {
		return ss[i].Value > ss[j].Value
	})

	return ss
}

func top5Alerts (a []alert.Alert) map[string]int {
	top := make(map[string]int)

	for i := range a {
		s := strings.Split(a[i].Message, " ")
		if len(s) > 2 {
			top[s[2]] = top[s[2]] + 1
		}
	}
	//if len(top) > 5 {
	// TODO: filter only 5 alert types
	//}
	return top
}

func razRaza (c int) string {
	s := strconv.Itoa(c)
	switch s[len(s)-1:] {
		case "2", "3", "4":
			return "раза"
		default:
			return "раз"
	}
}
func postSlackMessage (channel string, text string) error {
	pl := make(map[string]interface{})
	pl["channel"] = channel
	pl["text"] = text
	pl["username"] = "Максим"
	pl["mrkdwn"] = true

	d, err := json.Marshal(pl)
	if err != nil {
		return err
	}

	_, _, errors := gorequest.New().Post(os.Getenv("SLACK_HOOK")).Send(string(d)).End()
	if len(errors) > 0 {
		return errors[0]
	}
	return nil
}
