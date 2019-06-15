package store

import (
	"errors"
	"strings"

	"github.com/nlopes/slack"
)

var (
	log = map[string]LogData{} // key: "workspace,timestamp" // TODO: set limit?
)

type LogData struct {
	Channel string
	Body    string
}

var (
	ErrSlackLogNotFound     = errors.New("slack log is not found from cache")
	ErrSlackHistoryNotFound = errors.New("message history couldn't retrieve from GetConversationHistory")
)

func SetSlackLog(workspace, timestamp, channelName, text string) {
	// register post to kv
	// XXX: timestamp is unique only in channel
	k := strings.Join([]string{workspace, timestamp}, ",")

	// TODO: gc
	log[k] = LogData{
		Channel: channelName,
		Body:    text,
	}
}

func GetSlackLogFromCache(workspace, timestamp string) *LogData {
	parent := strings.Join([]string{workspace, timestamp}, ",")
	val, ok := log[parent]
	if !ok {
		return nil
	}
	return &val
}

func GetSlackLogFromAPI(api *slack.Client, workspace, channel, timestamp string) (*LogData, error) {
	params := &slack.GetConversationHistoryParameters{
		ChannelID: channel,
		Latest:    timestamp,
		Oldest:    timestamp,
		Inclusive: true,
		Limit:     1, // timestamp is guaranteed as unique (per channel)
	}

	history, err := api.GetConversationHistory(params)
	if err != nil {
		return nil, err
	}

	if len(history.Messages) != 1 {
		return nil, ErrSlackHistoryNotFound
	}

	msg := history.Messages[0]
	return &LogData{
		Channel: msg.Channel, // NOTE: this is not readable name
		Body:    msg.Text,
	}, nil
}
