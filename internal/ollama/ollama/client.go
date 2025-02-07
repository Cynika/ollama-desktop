package ollama

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"net/http"
	"net/url"
	"ollama-desktop/internal/config"
	"ollama-desktop/internal/ollama"
	"runtime"
	"strconv"
	"strings"
)

var noModelError = errors.New("No Model")

type Client struct {
	Base *url.URL
	Http *http.Client
}

func checkError(resp *http.Response, body []byte) error {
	if resp.StatusCode < http.StatusBadRequest {
		return nil
	}

	return ollama.StatusError{StatusCode: resp.StatusCode, Status: resp.Status, ErrorMessage: string(body)}
}

func (c *Client) do(ctx context.Context, path string, reqData map[string]string) ([]byte, error) {
	requestURL := c.Base.JoinPath(path)
	if len(reqData) > 0 {
		rawQuery := ""
		for name, value := range reqData {
			if rawQuery != "" {
				rawQuery += "&"
			}
			rawQuery += name + "=" + url.QueryEscape(value)
		}
		requestURL.RawQuery = rawQuery
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, err
	}

	request.Header.Set("User-Agent", fmt.Sprintf("ollama-desktop/%s (%s %s) Go/%s", config.BuildVersion, runtime.GOARCH, runtime.GOOS, runtime.Version()))

	respObj, err := c.Http.Do(request)
	if err != nil {
		return nil, err
	}
	defer respObj.Body.Close()

	respBody, err := io.ReadAll(respObj.Body)
	if err != nil {
		return nil, err
	}

	if err := checkError(respObj, respBody); err != nil {
		return nil, err
	}
	return respBody, nil
}

// 解析模型名称、是否归档
func parseNameArchive(s *goquery.Selection) (string, bool) {
	children := s.Children()
	name := strings.TrimSpace(children.Eq(0).Text())
	if children.Size() < 2 {
		return name, false
	}
	// 存在名称后面存在标签的情况，比如falcon
	for i := 1; i < children.Size(); i++ {
		if strings.Contains(children.Eq(i).Text(), "Archive") {
			return name, true
		}
	}
	return name, false
}

// 解析下载次数、标签数、更新时间
func parsePullTagCountAndUpdated(spans *goquery.Selection) (string, int, string) {
	pullCount := ""
	tagCount := 0
	updated := ""
	spans.Each(func(_ int, info *goquery.Selection) {
		text := strings.ReplaceAll(strings.TrimSpace(info.Text()), "\t", " ")
		if strings.HasSuffix(text, "Pulls") {
			pullCount = strings.TrimSpace(text[:len(text)-5])
		} else if strings.HasSuffix(text, "Pull") {
			pullCount = strings.TrimSpace(text[:len(text)-4])
		} else if strings.HasSuffix(text, "Tag") {
			tagCount, _ = strconv.Atoi(strings.TrimSpace(text[:len(text)-3]))
		} else if strings.HasSuffix(text, "Tags") {
			tagCount, _ = strconv.Atoi(strings.TrimSpace(text[:len(text)-4]))
		} else if strings.HasPrefix(text, "Updated") {
			updated = strings.TrimSpace(text[7:])
		}
	})
	return pullCount, tagCount, updated
}

func parseTags(spans *goquery.Selection) []string {
	var tags []string
	spans.Each(func(_ int, tag *goquery.Selection) {
		tags = append(tags, strings.TrimSpace(tag.Text()))
	})
	return tags
}

func (c *Client) Search(ctx context.Context, request *ollama.SearchRequest) ([]*ollama.ModelInfo, error) {
	respBody, err := c.do(ctx, "/search", map[string]string{
		"q": request.Q,
		"o": request.O,
		"c": request.C,
	})
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(respBody))
	if err != nil {
		return nil, err
	}
	var list []*ollama.ModelInfo

	// 解析模型信息
	doc.Find("ul.grid > li > a").Each(func(_ int, item *goquery.Selection) {
		name, archive := parseNameArchive(item.Find("div.flex > h2").First())
		description := strings.TrimSpace(item.Find("div.flex > p.break-words").First().Text())
		tags := parseTags(item.Find("div.flex > div.space-x-2 > span"))
		pullCount, tagCount, updated := parsePullTagCountAndUpdated(item.Find("div.flex > p.space-x-5 > span"))
		list = append(list, &ollama.ModelInfo{
			Name:        name,
			Archive:     archive,
			Description: description,
			Tags:        tags,
			PullCount:   pullCount,
			TagCount:    tagCount,
			UpdateTime:  updated,
		})
	})

	return list, err
}

func (c *Client) Library(ctx context.Context, request *ollama.LibraryRequest) ([]*ollama.ModelInfo, error) {
	if request.Sort == "" {
		request.Sort = "featured"
	}
	respBody, err := c.do(ctx, "/library", map[string]string{
		"q":    request.Q,
		"sort": request.Sort,
	})
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(respBody))
	if err != nil {
		return nil, err
	}
	var list []*ollama.ModelInfo

	// 解析模型信息
	doc.Find("ul.grid > li > a").Each(func(_ int, item *goquery.Selection) {
		name, archive := parseNameArchive(item.Find("div.flex > h2").First())
		description := strings.TrimSpace(item.Find("div.flex > p.break-words").First().Text())
		tags := parseTags(item.Find("div.space-y-2 > div.space-x-2 > span"))
		pullCount, tagCount, updated := parsePullTagCountAndUpdated(item.Find("div.space-y-2 > p.space-x-5 > span"))
		list = append(list, &ollama.ModelInfo{
			Name:        name,
			Archive:     archive,
			Description: description,
			Tags:        tags,
			PullCount:   pullCount,
			TagCount:    tagCount,
			UpdateTime:  updated,
		})
	})

	return list, err
}

func (c *Client) ModelTags(ctx context.Context, model string) (*ollama.ModelTagsResponse, error) {
	respBody, err := c.do(ctx, fmt.Sprintf("/library/%s/tags", model), nil)
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(respBody))
	if err != nil {
		return nil, err
	}

	name, archive := parseNameArchive(doc.Find("main > div.flex > div.flex > div.mb-3").First())
	description := strings.TrimSpace(doc.Find("main > div.flex #summary").First().Text())
	tags := parseTags(doc.Find("main > div.flex #summary+div > div.space-x-2 > span"))
	pullCount, tagCount, updated := parsePullTagCountAndUpdated(doc.Find("main > div.flex #summary+div > p.space-x-5 > span"))
	modelInfo := &ollama.ModelInfo{
		Name:        name,
		Archive:     archive,
		Description: description,
		Tags:        tags,
		PullCount:   pullCount,
		TagCount:    tagCount,
		UpdateTime:  updated,
	}

	response := &ollama.ModelTagsResponse{
		Model: modelInfo,
	}

	tagsNode := doc.Find("section > div > div > div.px-4.py-3 > div")

	if tagsNode.Size() < 1 {
		modelInfo.TagCount = 0
		return response, nil
	}

	var modelTags []*ollama.ModelTag
	for i := 0; i < tagsNode.Size(); i++ {
		tagNode := tagsNode.Eq(i)

		line1Node := tagNode.Find("div.space-x-2").First()
		name := strings.TrimSpace(line1Node.Find("a.group").Text())
		latest := strings.Contains(name, "latest")

		infos := strings.Split(tagNode.Find("div.space-x-1 > span").Text(), "•")
		tag := &ollama.ModelTag{
			Name:       name,
			Latest:     latest,
			Id:         strings.TrimSpace(infos[0]),
			Size:       strings.TrimSpace(infos[1]),
			UpdateTime: strings.TrimSpace(infos[2]),
		}
		modelTags = append(modelTags, tag)
	}
	response.Tags = modelTags
	modelInfo.TagCount = len(modelTags)

	return response, err
}

func (c *Client) ModelInfo(ctx context.Context, modelTag string) (*ollama.ModelInfoResponse, error) {
	respBody, err := c.do(ctx, fmt.Sprintf("/library/%s", modelTag), nil)
	if err != nil {
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(respBody))
	if err != nil {
		return nil, err
	}

	name, archive := parseNameArchive(doc.Find("main > div.flex > div.flex > div.mb-3").First())
	description := strings.TrimSpace(doc.Find("main > div.flex #summary").First().Text())
	tags := parseTags(doc.Find("main > div.flex #summary+div > div.space-x-2 > span"))
	pullCount, tagCount, updated := parsePullTagCountAndUpdated(doc.Find("main > div.flex #summary+div > p.space-x-5 > span"))
	if tagCount == 0 {
		// 模型详情页面排版变化，此处需要重新获取标签数量
		tagCountStr := strings.TrimSpace(doc.Find("main > div.flex section a[x-test-tags-link]").First().Text())
		if strings.HasSuffix(tagCountStr, "Tags") {
			tagCount, _ = strconv.Atoi(strings.TrimSpace(tagCountStr[:len(tagCountStr)-4]))
		} else if strings.HasSuffix(tagCountStr, "Tags") {
			tagCount, _ = strconv.Atoi(strings.TrimSpace(tagCountStr[:len(tagCountStr)-3]))
		}
	}
	modelInfo := &ollama.ModelInfo{
		Name:        name,
		Archive:     archive,
		Description: description,
		Tags:        tags,
		PullCount:   pullCount,
		TagCount:    tagCount,
		UpdateTime:  updated,
	}

	var modelMetas []*ollama.ModelMeta
	doc.Find("#file-explorer > section.py-2 > div a").Each(func(i int, selection *goquery.Selection) {
		children := selection.Children()
		name := strings.TrimSpace(children.Eq(0).Text())
		var content string
		// model 特殊处理，存在多部分内容
		if "model" == name {
			var values []string
			children.Eq(1).Find("div > span").Each(func(i int, span *goquery.Selection) {
				if i > 0 && i%2 == 0 {
					values = append(values, "·")
				}
				values = append(values, strings.TrimSpace(span.Text()))

			})
			content = strings.Join(values, " ")
		} else {
			content = strings.TrimSpace(children.Eq(1).Text())
		}
		meta := &ollama.ModelMeta{
			Name:    name,
			Content: content,
			Unit:    strings.TrimSpace(children.Eq(2).Text()),
			Href:    selection.AttrOr("href", ""),
		}
		modelMetas = append(modelMetas, meta)
	})

	readme := doc.Find("div#textareaInput > textarea#editor").Eq(0).Text()

	response := &ollama.ModelInfoResponse{
		Model: modelInfo,
		//Tags:   modelTags,
		Metas:  modelMetas,
		Readme: readme,
	}
	return response, err
}
