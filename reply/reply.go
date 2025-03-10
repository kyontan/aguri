package reply

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/nlopes/slack"
	"github.com/whywaita/aguri/config"
	"github.com/whywaita/aguri/store"
	"github.com/whywaita/slack_lib"
)

var (
	reChannel     = regexp.MustCompile(`(\S+)@([cdg]):(\S+)`)
	rePostChannel = regexp.MustCompile(`^(\S+)#(.*)$`)
	rePostIM      = regexp.MustCompile(`^(\S+)@(.*)$`)
	apiInstances  = map[string]*slack.Client{}
)

func validateMessage(fromType, aggrChannelName string, ev *slack.MessageEvent) bool {
	if !strings.Contains(aggrChannelName, config.PrefixSlackChannel) {
		// not aggr channel
		return false
	}

	if ev.Msg.User == "USLACKBOT" {
		return false
	}

	if ev.Msg.Text == "" {
		// not normal message
		return false
	}

	if fromType != "channel" && fromType != "group" {
		// TODO: implement other type
		return false
	}

	return true
}

func validateParsedMessage(userNames [][]string) bool {
	if len(userNames) == 0 {
		return false
	}

	return true
}

func getSlackApiInstance(workspaceName string) *slack.Client {
	api, ok := apiInstances[workspaceName]
	if ok == false {
		// not found
		api = slack.New(store.GetConfigFromAPI(workspaceName))
		apiInstances[workspaceName] = api
	}

	return api
}

func postReplyMessage(workspace string, ev *slack.MessageEvent, aggrChName string) error {
	api := getSlackApiInstance(workspace)
	logData := store.GetSlackLogFromCache(workspace, ev.ThreadTimestamp)
	channelToPost := ""
	if logData != nil {
		channelToPost = logData.Channel
	} else {
		log.Println("logData == nil, get from API")
		toApi := store.GetConfigToAPI()
		conversations, err := getConversations(toApi)
		if err != nil {
			return err
		}
		channelId := ""
		for _, conversation := range *conversations {
			if conversation.Name == aggrChName {
				channelId = conversation.ID
				break
			}
		}

		if channelId == "" {
			return fmt.Errorf("can't retrieve channelId of channel: %v", aggrChName)
		}

		parentMsg, errFromAPI := store.GetSlackMessageFromAPI(toApi, channelId, ev.ThreadTimestamp)
		if errFromAPI != nil {
			return errFromAPI
		}

		userName := reChannel.FindStringSubmatch(parentMsg.Username)
		if len(userName) != 4 {
			return fmt.Errorf("can't parse userName: %v", parentMsg.Username)
		}

		channelType := userName[2] // c or d or g
		channelName := userName[3]
		switch channelType {
		case "c":
			channelToPost = "#" + channelName
		case "g":
			channelToPost = channelName
		case "d":
			userId, err := getUserIDfromDisplayName(api, channelName)
			if err != nil {
				return err
			}
			channelToPost = *userId
		}
	}

	// Post
	param := slack.PostMessageParameters{
		AsUser: true,
	}

	_, _, err := api.PostMessage(channelToPost, slack.MsgOptionText(ev.Text, false), slack.MsgOptionPostMessageParameters(param))
	return err
}

func getConversations(api *slack.Client) (*[]slack.Channel, error) {
	params := &slack.GetConversationsParameters{
		Types:           []string{"public_channel", "private_channel"},
		ExcludeArchived: "true",
	}

	var conversations []slack.Channel
	conversations, next_cursor, err := api.GetConversations(params)

	if err != nil {
		return nil, err
	}

	for next_cursor != "" {
		var conversationsToAppend []slack.Channel
		params.Cursor = next_cursor
		conversationsToAppend, next_cursor, err = api.GetConversations(params)

		if err != nil {
			return nil, err
		}

		conversations = append(conversations, conversationsToAppend...)
	}

	return &conversations, nil
}

// public / private channel
func PostNewMessageToChannel(workspace string, ev *slack.MessageEvent) (bool, error) {
	postChannelMatches := rePostChannel.FindStringSubmatch(ev.Text)
	if len(postChannelMatches) != 3 { // [whole text, post_to, body]
		return false, nil
	}

	postTo := postChannelMatches[1]

	// NOTE: to and from is reversed when posting
	toApi := getSlackApiInstance(workspace)
	fromApi := store.GetConfigToAPI()

	conversations, err := getConversations(getSlackApiInstance(workspace))
	if err != nil {
		return true, err
	}

	var candidacy []string
	for _, conversation := range *conversations {
		if strings.HasPrefix(conversation.Name, postTo) {
			if conversation.IsChannel && !conversation.IsPrivate {
				candidacy = append(candidacy, "#"+conversation.Name)
			} else {
				candidacy = append(candidacy, conversation.Name)
			}
		}
	}

	if len(candidacy) == 1 {
		param := slack.PostMessageParameters{
			AsUser: true,
		}
		body := postChannelMatches[2]
		_, _, err = toApi.PostMessage(candidacy[0], slack.MsgOptionText(body, false), slack.MsgOptionPostMessageParameters(param))

		if err != nil {
			return true, err
		}

		_, _, err = fromApi.DeleteMessage(ev.Channel, ev.Timestamp)
	} else if len(candidacy) == 0 {
		param := slack.PostMessageParameters{
			Username: "aguri",
		}

		msg := fmt.Sprintf("Not found channel: %v", postTo)

		_, _, err = fromApi.PostMessage(config.PrefixSlackChannel+workspace, slack.MsgOptionText(msg, false), slack.MsgOptionPostMessageParameters(param))
	} else {
		param := slack.PostMessageParameters{
			Username: "aguri",
		}

		msg := fmt.Sprintf("Found multiple candidacies: %v", strings.Join(candidacy, ", "))

		_, _, err = fromApi.PostMessage(config.PrefixSlackChannel+workspace, slack.MsgOptionText(msg, false), slack.MsgOptionPostMessageParameters(param))
	}

	return true, err
}

func getUserIDfromDisplayName(api *slack.Client, name string) (*string, error) {
	users, err := api.GetUsers()
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		if user.Profile.DisplayName == name {
			return &user.ID, nil
		}
	}
	return nil, nil
}

func PostNewMessageToIM(workspace string, ev *slack.MessageEvent) (bool, error) {
	postIMMatches := rePostIM.FindStringSubmatch(ev.Text)
	if len(postIMMatches) != 3 { // [whole text, post_to, body]
		return false, nil
	}

	postTo := postIMMatches[1]

	// NOTE: to and from is reversed when posting
	toApi := getSlackApiInstance(workspace)
	fromApi := store.GetConfigToAPI()

	userId, err := getUserIDfromDisplayName(toApi, postTo)
	if err != nil {
		return true, err
	}

	if userId == nil {
		param := slack.PostMessageParameters{
			Username: "aguri",
		}

		msg := fmt.Sprintf("Not found username: %v", postTo)

		_, _, err = fromApi.PostMessage(config.PrefixSlackChannel+workspace, slack.MsgOptionText(msg, false), slack.MsgOptionPostMessageParameters(param))
		return true, err
	}

	param := slack.PostMessageParameters{
		AsUser: true,
	}
	body := postIMMatches[2]
	_, _, err = toApi.PostMessage(*userId, slack.MsgOptionText(body, false), slack.MsgOptionPostMessageParameters(param))
	if err != nil {
		return true, err
	}

	_, _, err = fromApi.DeleteMessage(ev.Channel, ev.Timestamp)
	return true, err
}

func PostNewMessage(workspace string, ev *slack.MessageEvent) error {
	ok, err := PostNewMessageToChannel(workspace, ev)

	if ok {
		return err
	}

	_, err = PostNewMessageToIM(workspace, ev)
	return err
}

func HandleReplyMessage() {
	toAPI := store.GetConfigToAPI()
	rtm := toAPI.NewRTM()
	go rtm.ManageConnection()
	for msg := range rtm.IncomingEvents {
		switch ev := msg.Data.(type) {
		case *slack.MessageEvent:
			if ev.SubType == "bot_message" {
				break
			}

			fromType, aggrChName, err := slack_lib.ConvertDisplayChannelName(toAPI, ev)
			if err != nil {
				log.Println(err)
				break
			}
			if !validateMessage(fromType, aggrChName, ev) {
				// invalid message
				break
			}

			workspace := strings.TrimPrefix(aggrChName, config.PrefixSlackChannel)

			if ev.ThreadTimestamp != "" {
				// message posted as thread reply
				err := postReplyMessage(workspace, ev, aggrChName)
				if err != nil {
					log.Println(err)
					break
				}
			}

			if ev.Username == "" {
				err = PostNewMessage(workspace, ev)
				if err != nil {
					log.Println(err)
				}
				break
			}

			// Expect ev is posted message from aguri
			userName := reChannel.FindStringSubmatch(ev.Username)
			if len(userName) != 4 {
				log.Printf("can't get source channel name from: %v", userName)
				break
			}

			chReadableName := userName[3]
			store.SetSlackLog(workspace, ev.Timestamp, chReadableName, ev.Text)

		case *slack.RTMError:
			fmt.Printf("Error: %s\n", ev.Error())

		default:
			// Ignore
		}

	}
}
