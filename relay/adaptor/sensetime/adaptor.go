package sensetime

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/relay/adaptor"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/meta"
	"github.com/songquanpeng/one-api/relay/model"
	"github.com/songquanpeng/one-api/relay/relaymode"
)

type Adaptor struct {
}

func (a *Adaptor) Init(meta *meta.Meta) {

}

func (a *Adaptor) GetRequestURL(meta *meta.Meta) (string, error) {
	if strings.HasPrefix(meta.ActualModelName, "Embedding") {
		return fmt.Sprintf("%s/v1/llm/embeddings", meta.BaseURL), nil
	}

	return fmt.Sprintf("%s/v1/llm/chat-completions", meta.BaseURL), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *meta.Meta) error {
	parts := strings.Split(meta.APIKey, "|")
	if len(parts) != 2 {
		return errors.New("invalid_auth")
	}

	accesskey := parts[0]
	secretKey := parts[1]

	adaptor.SetupCommonRequestHeader(c, req, meta)
	token := GetToken(accesskey, secretKey)
	req.Header.Set("Authorization", token)
	return nil
}

func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *model.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	switch relayMode {
	case relaymode.Embeddings:
		embeddingrequest := ConvertEmbeddingRequest(*request)
		return embeddingrequest, nil
	default:
		senserequest := ConvertRequest(*request)
		return senserequest, nil
	}
}

func (a *Adaptor) ConvertImageRequest(request *model.ImageRequest) (any, error) {
	return nil, errors.New("request is not supported")
}

func (a *Adaptor) DoRequest(c *gin.Context, meta *meta.Meta, requestBody io.Reader) (*http.Response, error) {
	return adaptor.DoRequestHelper(a, c, meta, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, meta *meta.Meta) (usage *model.Usage, err *model.ErrorWithStatusCode) {
	switch meta.Mode {
	case relaymode.Embeddings:
		err, usage = EmbeddingsHandler(c, resp)
		return
	case relaymode.ImagesGenerations:
		err, usage = openai.ImageHandler(c, resp)
		return
	}

	if meta.IsStream {
		err, usage = StreamHandler(c, resp)
	} else {
		if meta.Mode == relaymode.Embeddings {
			err, usage = EmbeddingsHandler(c, resp)
		} else {
			err, usage = Handler(c, resp)
		}
	}
	return
}

func ConvertEmbeddingRequest(request model.GeneralOpenAIRequest) *EmbeddingRequest {
	return &EmbeddingRequest{
		Model: "nova-embedding-stable",
		Input: request.ParseInput(),
	}
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return "sensetime"
}
