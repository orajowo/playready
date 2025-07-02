package playready

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/xml"
	"fmt"
)

type wrmHeader struct {
	XMLName xml.Name `xml:"WRMHEADER"`
	XMLNS   string   `xml:"xmlns,attr"`
	Version string   `xml:"version,attr"`
	Data    wrmData  `xml:"DATA"`
}

type wrmData struct {
	ProtectInfo *wrmProtectInfo `xml:"PROTECTINFO"`
	KID         string          `xml:"KID"`
	Checksum    string          `xml:"CHECKSUM"`
}

type wrmProtectInfo struct {
	KeyLen string `xml:"KEYLEN"`
	AlgID  string `xml:"ALGID"`
}

func BuildWRMHeader(kidB64 string) (string, error) {
	kidBytes, err := base64.StdEncoding.DecodeString(kidB64)
	if err != nil {
		return "", fmt.Errorf("invalid base64 KID: %v", err)
	}
	checksumBytes := sha1.Sum(kidBytes)
	checksum := base64.StdEncoding.EncodeToString(checksumBytes[:8])

	header := wrmHeader{
		XMLNS:   "http://schemas.microsoft.com/DRM/2007/03/PlayReadyHeader",
		Version: "4.0.0.0",
		Data: wrmData{
			ProtectInfo: &wrmProtectInfo{KeyLen: "16", AlgID: "AESCTR"},
			KID:         kidB64,
			Checksum:    checksum,
		},
	}

	buf := &bytes.Buffer{}
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(buf)
	enc.Indent("", "   ")
	if err := enc.Encode(header); err != nil {
		return "", err
	}
	return buf.String(), nil
}
