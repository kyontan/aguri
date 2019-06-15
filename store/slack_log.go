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

func GetSlackMessageFromAPI(api *slack.Client, channelId, timestamp string) (*slack.Message, error) {
	params := &slack.GetConversationHistoryParameters{
		ChannelID: channelId,
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

	return &history.Messages[0], nil
}
