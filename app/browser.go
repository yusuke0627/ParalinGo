package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/egoist/mygo"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Browser owns an isolated external-page window. External origins cannot call Go services.
type Browser struct {
	mu     sync.Mutex
	window *mygo.Window
}
type WebArticle struct {
	Title      string   `json:"title"`
	Source     string   `json:"source"`
	Paragraphs []string `json:"paragraphs"`
}

func validateWebURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return "", fmt.Errorf("http または https のURLを入力してください")
	}
	return u.String(), nil
}
func (b *Browser) Open(raw string) error {
	target, err := validateWebURL(raw)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.window == nil {
		win := mygo.NewWindow(mygo.WindowOptions{Title: "ParalinGo · Browser", URL: target, Width: 1100, Height: 800})
		b.window = win
		win.OnClosed(func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			if b.window == win {
				b.window = nil
			}
		})
	} else {
		if err := b.window.Page().LoadURL(target); err != nil {
			return err
		}
		b.window.Show()
	}
	return nil
}
func (b *Browser) Navigate(action string) error {
	b.mu.Lock()
	win := b.window
	b.mu.Unlock()
	if win == nil {
		return fmt.Errorf("先にURLを開いてください")
	}
	switch action {
	case "back":
		win.Page().GoBack()
	case "forward":
		win.Page().GoForward()
	case "reload":
		win.Page().Reload()
	default:
		return fmt.Errorf("不明な操作です")
	}
	win.Show()
	return nil
}
func (b *Browser) Capture() (WebArticle, error) {
	b.mu.Lock()
	win := b.window
	b.mu.Unlock()
	if win == nil {
		return WebArticle{}, fmt.Errorf("先にURLを開いてください")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	value, err := win.Page().EvalContext(ctx, extractArticleScript)
	if err != nil {
		return WebArticle{}, fmt.Errorf("本文を取得できませんでした: %w", err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return WebArticle{}, err
	}
	var result WebArticle
	if err = json.Unmarshal(data, &result); err != nil {
		return result, err
	}
	if _, err = validateWebURL(result.Source); err != nil {
		return WebArticle{}, err
	}
	if len(result.Paragraphs) == 0 {
		return result, fmt.Errorf("本文が見つかりません。ページの読み込み後に再試行してください")
	}
	return result, nil
}

const extractArticleScript = `(() => {
 if (document.readyState === 'loading') throw Error('ページ読み込み中です');
 const visible = el => { const style = getComputedStyle(el); return style.display !== 'none' && style.visibility !== 'hidden' && el.getClientRects().length > 0; };
 const candidates = [...document.querySelectorAll('article, main, [role="main"]')].filter(visible);
 const score = el => [...el.querySelectorAll('p')].filter(visible).reduce((n,p)=>n+(p.innerText||'').length,0);
 const container = candidates.sort((a,b)=>score(b)-score(a))[0] || document.body;
 const paragraphs = [...container.querySelectorAll('p')].filter(el=>visible(el) && !el.closest('nav, footer, header, aside, form, [role="navigation"]')).map(el=>(el.innerText||'').replace(/\s+/g,' ').trim()).filter(text=>text.length>20);
 if (paragraphs.join('').length > 500000) throw Error('本文が長すぎます');
 return {title: document.title || location.hostname, source: location.href, paragraphs};
})()`
