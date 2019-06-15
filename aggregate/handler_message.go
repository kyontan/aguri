package aggregate

import (
	"bufio"
	"fmt"
	"log"
	"strings"

	"github.com/whywaita/aguri/store"

	"github.com/nlopes/slack"
	"github.com/whywaita/aguri/config"
	"github.com/whywaita/aguri/utils"
)

func HandleMessageEvent(ev *slack.MessageEvent, fromAPI *slack.Client, workspace, lastTimestamp string) string {
	var err error

	if lastTimestamp != ev.Timestamp {
		// if lastTimestamp == eve.Timestamp, that message is same.
		toChannelName := config.PrefixSlackChannel + strings.ToLower(workspace)

		switch ev.SubType {
		case "message_changed":
			err = HandleMessageEdited(ev, fromAPI, workspace, toChannelName)
			if err != nil {
				log.Println(err)
			}
		case "message_deleted":
			err = HandleMessageDeleted(ev, fromAPI, workspace, toChannelName)
			if err != nil {
				log.Println(err)
			}
		case "":
			err = utils.PostMessageToChannel(store.GetConfigToAPI(), fromAPI, ev, ev.Text, toChannelName)
			if err != nil {
				log.Println(err)
			}
		default:
			text := "SubType: " + ev.SubType + "\n" + ev.Text
			err = utils.PostMessageToChannel(store.GetConfigToAPI(), fromAPI, ev, text, toChannelName)
			if err != nil {
				log.Println(err)
			}
		}

		return ev.Timestamp
	}

	return lastTimestamp
}

func HandleMessageDeleted(ev *slack.MessageEvent, fromAPI *slack.Client, workspace, toChannelName string) error {
	oldLog := store.GetSlackLogFromCache(workspace, ev.DeletedTimestamp)
	msg := ""

	if oldLog != nil {
		if countLines(oldLog.Body) == 1 {
			msg = fmt.Sprintf("Original Text: %v", oldLog.Body)
		} else {
			msg = fmt.Sprintf("Original Text:\n%v", oldLog.Body)
		}
	} else {
		msg = "Deleted unknown message"
	}

	err := utils.PostMessageToChannel(store.GetConfigToAPI(), fromAPI, ev, msg, toChannelName)
	if err != nil {
		return err
	}

	return nil
}

func countLines(s string) int {
	scanner := bufio.NewScanner(strings.NewReader(s))
	lines := 0
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		lines++
	}
	return lines
}

func HandleMessageEdited(ev *slack.MessageEvent, fromAPI *slack.Client, workspace, toChannelName string) error {
	oldLog := store.GetSlackLogFromCache(workspace, ev.SubMessage.Timestamp)
	msg := ""

	if oldLog != nil {
		msg += "Edited from: "
		if countLines(oldLog.Body) == 1 {
			msg += oldLog.Body
		} else {
			msg += "\n"
			msg += oldLog.Body
		}
	} else {
		msg += "Edited from: (unknown)"
	}
	msg += "\n"

	msg += "Edited to: "
	if countLines(ev.SubMessage.Text) == 1 {
		msg += ev.SubMessage.Text
	} else {
		msg += "\n"
		msg += ev.SubMessage.Text
	}

	err := utils.PostMessageToChannel(store.GetConfigToAPI(), fromAPI, ev, msg, toChannelName)
	if err != nil {
		return err
	}

	return nil
}
