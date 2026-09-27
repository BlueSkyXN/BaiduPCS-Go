package pcsupload

import (
	"context"
	"github.com/qjfoidnh/BaiduPCS-Go/baidupcs"
	"github.com/qjfoidnh/BaiduPCS-Go/baidupcs/pcserror"
	"github.com/qjfoidnh/BaiduPCS-Go/internal/pcsconfig"
	"github.com/qjfoidnh/BaiduPCS-Go/requester"
	"github.com/qjfoidnh/BaiduPCS-Go/requester/multipartreader"
	"github.com/qjfoidnh/BaiduPCS-Go/requester/rio"
	"github.com/qjfoidnh/BaiduPCS-Go/requester/uploader"
	"io"
	"net/http"
	"sync"
	"time"
)

type (
	PCSUpload struct {
		pcs        *baidupcs.BaiduPCS
		targetPath string
	}

	EmptyReaderLen64 struct {
	}
)

// uploadClients 按 cookie jar(即按账号)缓存上传客户端, 避免单例客户端
// 把第一个请求者的 cookie jar 钉死给所有账号
var uploadClients sync.Map // http.CookieJar -> *requester.HTTPClient

func getUploadClient(jar http.CookieJar) *requester.HTTPClient {
	if c, ok := uploadClients.Load(jar); ok {
		return c.(*requester.HTTPClient)
	}
	cli := pcsconfig.Config.PCSHTTPClient()
	cli.SetCookiejar(jar)
	cli.SetTimeout(200 * time.Second)
	actual, _ := uploadClients.LoadOrStore(jar, cli)
	return actual.(*requester.HTTPClient)
}

var pcsPeriod = 256 // 上传多少个分片更换一次pcsHost

func (e EmptyReaderLen64) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

func (e EmptyReaderLen64) Len() int64 {
	return 0
}

func NewPCSUpload(pcs *baidupcs.BaiduPCS, targetPath string) uploader.MultiUpload {
	return &PCSUpload{
		pcs:        pcs,
		targetPath: targetPath,
	}
}

func (pu *PCSUpload) lazyInit() {
	if pu.pcs == nil {
		pu.pcs = &baidupcs.BaiduPCS{}
	}
}

// Precreate 上传前准备, 获取本次上传用的pcs服务器
func (pu *PCSUpload) Precreate() (originPCSHost string, pcsError pcserror.Error) {
	originPCSHost = pu.pcs.GetPCSAddr()
	_, newPCSHost := pu.pcs.GetRandomPCSHost()
	pu.pcs.SetPCSAddr(newPCSHost)
	return
}

func (pu *PCSUpload) TmpFile(ctx context.Context, uploadId, targetPath string, partSeq int, partOffset int64, r rio.ReaderLen64) (checksum string, uperr error) {
	pu.lazyInit()

	var respErr *uploader.MultiError

	// 每上传一定量分片切换一次动态pcs addr
	if partSeq%pcsPeriod == pcsPeriod-1 {
		go func() {
			_, newPCSHost := pu.pcs.GetRandomPCSHost()
			pu.pcs.SetPCSAddr(newPCSHost)
		}()
	}

	checksum, pcsError := pu.pcs.UploadTmpFile(uploadId, targetPath, partSeq, partOffset, func(uploadURL string, jar http.CookieJar) (resp *http.Response, err error) {

		mr := multipartreader.NewMultipartReader()
		mr.AddFormFile("uploadedfile", "", r)
		mr.CloseMultipart()

		doneChan := make(chan struct{}, 1)
		go func() {
			cli := getUploadClient(jar)
			resp, err = cli.Req(http.MethodPost, uploadURL, mr, nil)
			doneChan <- struct{}{}

			if resp != nil {
				// 不可恢复的错误
				switch resp.StatusCode {
				case 400, 401, 403, 413: // 4xx通常是由客户端非法操作引发，直接深度重试
					respErr = &uploader.MultiError{
						Terminated: true,
					}
				}
			}
		}()
		select {
		case <-ctx.Done(): // 取消
			// 返回, 让那边关闭连接
			return resp, ctx.Err()
		case <-doneChan:
			// return
		}
		return
	})

	if respErr != nil {
		respErr.Err = pcsError
		return checksum, respErr
	}

	return checksum, pcsError
}

func (pu *PCSUpload) CreateSuperFile(pcsHost, policy, uploadId string, fileSize int64, checksumMap map[int]string) (err error) {
	pu.lazyInit()
	pu.pcs.SetPCSAddr(pcsHost) // 恢复默认pcs服务器
	return pu.pcs.UploadCreateSuperFile(uploadId, policy, fileSize, pu.targetPath, checksumMap)
}
