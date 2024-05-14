package sensetime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/constant"
	"github.com/songquanpeng/one-api/relay/model"
)

func GetToken(ak string, sk string) string {
	payload := jwt.MapClaims{
		"iss": ak,
		"exp": time.Now().Add(1800 * time.Second).Unix(),
		"nbf": time.Now().Add(-5 * time.Second).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)
	signedToken, err := token.SignedString([]byte(sk))
	if err != nil {
		fmt.Println("Error encoding JWT token:", err)
		return ""
	}
	return signedToken
}

func ConvertRequest(request model.GeneralOpenAIRequest) *ChatRequest {
	messages := make([]Message, 0, len(request.Messages))
	for _, message := range request.Messages {
		messages = append(messages, Message{
			Role:    message.Role,
			Content: message.StringContent(),
		})
	}

	senseRequest := ChatRequest{
		Model:        request.Model,
		Messages:     messages,
		Temperature:  request.Temperature,
		TopP:         request.TopP,
		Stream:       request.Stream,
		MaxNewTokens: request.MaxTokens,
	}

	fmt.Printf("[DEBUG] request backend llm: %+v\n", senseRequest)

	return &senseRequest
}

func responseSenseChat2OpenAI(response *ChatResponse) *openai.TextResponse {
	choice := openai.TextResponseChoice{
		Index: 0,
		Message: model.Message{
			Role:    "assistant",
			Content: response.Data.Choices[0].Message,
		},
		FinishReason: "stop",
	}
	fullTextResponse := openai.TextResponse{
		Id:      response.Data.Id,
		Model:   "sensechat",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Choices: []openai.TextResponseChoice{choice},
		Usage:   response.Data.Usage,
	}
	return &fullTextResponse
}

func streamResponseSenseChat2OpenAI(senseChatResponse *ChatStreamResponse) *openai.ChatCompletionsStreamResponse {
	var choice openai.ChatCompletionsStreamResponseChoice
	choice.Delta.Content = senseChatResponse.Data.Choices[0].Delta
	if senseChatResponse.Data.Choices[0].FinishReason == "stop" {
		choice.FinishReason = &constant.StopFinishReason
	}
	response := openai.ChatCompletionsStreamResponse{
		Id:      senseChatResponse.Data.Id,
		Object:  "chat.completion.chunk",
		Created: helper.GetTimestamp(),
		Model:   "sensechat",
		Choices: []openai.ChatCompletionsStreamResponseChoice{choice},
	}
	return &response
}

func StreamHandler(c *gin.Context, resp *http.Response) (*model.ErrorWithStatusCode, *model.Usage) {
	var usage model.Usage
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		//fmt.Printf("[DEBUG] received stream buffer: %s", string(data))
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := strings.Index(string(data), "\n\n"); i >= 0 && strings.Index(string(data), ":") >= 0 {
			return i + 2, data[0:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	})
	dataChan := make(chan string)
	stopChan := make(chan bool)
	go func() {
		for scanner.Scan() {
			data := scanner.Text()
			if len(data) < 5 { // ignore blank line or wrong format
				continue
			}
			data = data[5:]
			dataChan <- data
		}
		stopChan <- true
	}()
	common.SetEventStreamHeaders(c)
	c.Stream(func(w io.Writer) bool {
		select {
		case data := <-dataChan:
			var senseChatResponse ChatStreamResponse
			err := json.Unmarshal([]byte(data), &senseChatResponse)
			if err != nil {
				logger.SysError("error unmarshalling stream response: " + err.Error())
				return true
			}
			if senseChatResponse.Usage.TotalTokens != 0 {
				usage.TotalTokens = senseChatResponse.Usage.TotalTokens
				usage.PromptTokens = senseChatResponse.Usage.PromptTokens
				usage.CompletionTokens = senseChatResponse.Usage.CompletionTokens
			}
			response := streamResponseSenseChat2OpenAI(&senseChatResponse)
			jsonResponse, err := json.Marshal(response)
			if err != nil {
				logger.SysError("error marshalling stream response: " + err.Error())
				return true
			}
			c.Render(-1, common.CustomEvent{Data: "data: " + string(jsonResponse)})
			return true
		case <-stopChan:
			c.Render(-1, common.CustomEvent{Data: "data: [DONE]"})
			return false
		}
	})
	err := resp.Body.Close()
	if err != nil {
		return openai.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), nil
	}
	return nil, &usage
}

func Handler(c *gin.Context, resp *http.Response) (*model.ErrorWithStatusCode, *model.Usage) {
	var senseChatResponse ChatResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError), nil
	}
	err = resp.Body.Close()
	if err != nil {
		return openai.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), nil
	}
	err = json.Unmarshal(responseBody, &senseChatResponse)
	if err != nil {
		return openai.ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError), nil
	}
	if senseChatResponse.Error.Message != "" {
		return &model.ErrorWithStatusCode{
			Error: model.Error{
				Message: senseChatResponse.Error.Message,
				Type:    "seneseChat_error",
				Param:   "",
				Code:    senseChatResponse.Error.Code,
			},
			StatusCode: resp.StatusCode,
		}, nil
	}
	fullTextResponse := responseSenseChat2OpenAI(&senseChatResponse)
	fullTextResponse.Model = "sensechat"
	jsonResponse, err := json.Marshal(fullTextResponse)
	if err != nil {
		return openai.ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError), nil
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, err = c.Writer.Write(jsonResponse)
	return nil, &fullTextResponse.Usage
}

func EmbeddingsHandler(c *gin.Context, resp *http.Response) (*model.ErrorWithStatusCode, *model.Usage) {
	var senseChatResponse EmbeddingResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return openai.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError), nil
	}
	err = resp.Body.Close()
	if err != nil {
		return openai.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), nil
	}
	err = json.Unmarshal(responseBody, &senseChatResponse)
	if err != nil {
		return openai.ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError), nil
	}
	fullTextResponse := embeddingResponseSenseChat2OpenAI(&senseChatResponse)
	jsonResponse, err := json.Marshal(fullTextResponse)
	if err != nil {
		return openai.ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError), nil
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, err = c.Writer.Write(jsonResponse)
	return nil, &fullTextResponse.Usage
}

func embeddingResponseSenseChat2OpenAI(response *EmbeddingResponse) *openai.EmbeddingResponse {
	openAIEmbeddingResponse := openai.EmbeddingResponse{
		Object: "list",
		Data:   make([]openai.EmbeddingResponseItem, 0, len(response.Embeddings)),
		Model:  "nova-embedding-stable",
		Usage: model.Usage{
			PromptTokens:     response.Usage.PromptTokens,
			CompletionTokens: response.Usage.TotalTokens - response.Usage.PromptTokens,
			TotalTokens:      response.Usage.TotalTokens,
		},
	}

	for _, item := range response.Embeddings {
		openAIEmbeddingResponse.Data = append(openAIEmbeddingResponse.Data, openai.EmbeddingResponseItem{
			Object:    `embedding`,
			Index:     item.Index,
			Embedding: item.Embedding,
		})
	}
	return &openAIEmbeddingResponse
}
