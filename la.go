package playready

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/orajowo/etree"
	"github.com/orajowo/playready/crypto"
)

func (Challenge) CipherData(certChain Chain, key crypto.XmlKey) ([]byte, error) {
	doc := etree.NewDocument()
	doc.WriteSettings.CanonicalEndTags = true
	doc.CreateChild("Data", func(e *etree.Element) {
		e.CreateChild("CertificateChains", func(e *etree.Element) {
			e.CreateChild("CertificateChain", func(e *etree.Element) {
				e.CreateText(" " + base64.StdEncoding.EncodeToString(certChain.Encode()) + " ")
			})
		})
		e.CreateChild("Features", func(e *etree.Element) {
			e.CreateChild("Feature", func(e *etree.Element) {
				e.CreateAttr("Name", "AESCBC")
			})
		})
	})
	doc.Indent(0)
	base, err := doc.WriteToString()
	if err != nil {
		return nil, err
	}
	base = strings.Replace(base, "\n", "", -1)
	var Aes crypto.Aes
	ciphertext, err := Aes.EncryptCBC(key, base)
	if err != nil {
		return nil, err
	}
	return append(key.AesIv[:], ciphertext...), nil
}

func (Challenge) SignedInfo(digest []byte) *etree.Document {
	doc := etree.NewDocument()
	doc.WriteSettings.CanonicalEndTags = true

	doc.CreateChild("SignedInfo", func(e *etree.Element) {
		e.CreateAttr("xmlns", "http://www.w3.org/2000/09/xmldsig#")
		e.CreateChild("CanonicalizationMethod", func(e *etree.Element) {
			e.CreateAttr("Algorithm", "http://www.w3.org/TR/2001/REC-xml-c14n-20010315")
		})
		e.CreateChild("SignatureMethod", func(e *etree.Element) {
			e.CreateAttr("Algorithm", "http://schemas.microsoft.com/DRM/2007/03/protocols#ecdsa-sha256")
		})
		e.CreateChild("Reference", func(e *etree.Element) {
			e.CreateAttr("URI", "#SignedData")
			e.CreateChild("DigestMethod", func(e *etree.Element) {
				e.CreateAttr("Algorithm", "http://schemas.microsoft.com/DRM/2007/03/protocols#sha256")
			})
			e.CreateChild("DigestValue", func(e *etree.Element) {
				e.CreateText(base64.StdEncoding.EncodeToString(digest))
			})
		})
	})

	doc.Indent(0)
	return doc
}
func (Challenge) LicenseAcquisition(
	key crypto.XmlKey, cipherData []byte, header Header,
) (*etree.Document, error) {
	doc := etree.NewDocument()
	doc.WriteSettings.CanonicalEndTags = true
	LicenseNonce := make([]byte, 16)
	_, err := rand.Read(LicenseNonce)
	if err != nil {
		return nil, err
	}
	var WMRMEccPubKey crypto.WMRM
	x, y, err := WMRMEccPubKey.Points()
	if err != nil {
		return nil, err
	}
	var LicenseVersion string
	switch header.WrmHeader.Version {
	case "4.2.0.0":
		LicenseVersion = "4"
	case "4.3.0.0":
		LicenseVersion = "5"
	default:
		LicenseVersion = "1"
	}
	var elgamal crypto.ElGamal
	doc.WriteSettings.CanonicalEndTags = true
	doc.CreateChild("LA", func(e *etree.Element) {
		e.CreateAttr("xmlns", "http://schemas.microsoft.com/DRM/2007/03/protocols")
		e.CreateAttr("Id", "SignedData")
		e.CreateAttr("xml:space", "preserve")
		e.CreateChild("Version", func(e *etree.Element) {
			e.CreateText(LicenseVersion)
		})
		e.CreateChild("ContentHeader", func(e *etree.Element) {
			e.AddChild(header.WrmHeader.Data)
		})
		e.CreateChild("CLIENTINFO", func(e *etree.Element) {
			e.CreateChild("CLIENTVERSION", func(e *etree.Element) {
				e.CreateText("4.0.1.2")
			})
		})

		e.CreateChild("LicenseNonce", func(e *etree.Element) {
			e.CreateText(base64.StdEncoding.EncodeToString(LicenseNonce))
		})
		e.CreateChild("ClientTime", func(e *etree.Element) {
			e.CreateText(strconv.FormatInt(time.Now().Unix(), 10))
		})
		e.CreateChild("EncryptedData", func(e *etree.Element) {
			e.CreateAttr("xmlns", "http://www.w3.org/2001/04/xmlenc#")
			e.CreateAttr("Type", "http://www.w3.org/2001/04/xmlenc#Element")
			e.CreateChild("EncryptionMethod", func(e *etree.Element) {
				e.CreateAttr("Algorithm", "http://www.w3.org/2001/04/xmlenc#aes128-cbc")
			})
			e.CreateChild("KeyInfo", func(e *etree.Element) {
				e.CreateAttr("xmlns", "http://www.w3.org/2000/09/xmldsig#")
				e.CreateChild("EncryptedKey", func(e *etree.Element) {
					e.CreateAttr("xmlns", "http://www.w3.org/2001/04/xmlenc#")
					e.CreateChild("EncryptionMethod", func(e *etree.Element) {
						e.CreateAttr("Algorithm", "http://schemas.microsoft.com/DRM/2007/03/protocols#ecc256")
					})
					e.CreateChild("KeyInfo", func(e *etree.Element) {
						e.CreateAttr("xmlns", "http://www.w3.org/2000/09/xmldsig#")
						e.CreateChild("KeyName", func(e *etree.Element) {
							e.CreateText("WMRMServer")
						})
					})
					e.CreateChild("CipherData", func(e *etree.Element) {
						e.CreateChild("CipherValue", func(e *etree.Element) {
							var encrypted []byte
							encrypted, err = elgamal.Encrypt(x, y, key)
							e.CreateText(base64.StdEncoding.EncodeToString(encrypted))
						})
					})
				})
			})
			e.CreateChild("CipherData", func(e *etree.Element) {
				e.CreateChild("CipherValue", func(e *etree.Element) {
					e.CreateText(base64.StdEncoding.EncodeToString(cipherData))
				})
			})
		})
	})
	if err != nil {
		return nil, err
	}
	doc.Indent(0)
	return doc, nil
}

type Challenge struct{}

func (c Challenge) Create(
	certificateChain Chain, signingKey crypto.EcKey, header Header,
) (string, error) {
	var key crypto.XmlKey
	err := key.New()
	if err != nil {
		return "", err
	}
	cipherData, err := c.CipherData(certificateChain, key)
	if err != nil {
		return "", err
	}
	LA, err := c.LicenseAcquisition(key, cipherData, header)
	if err != nil {
		return "", err
	}
	LAStr, err := LA.WriteToString()
	if err != nil {
		return "", err
	}
	LAStr = strings.Replace(LAStr, "\n", "", -1)
	LADigest := sha256.Sum256([]byte(LAStr))
	SignedInfo := c.SignedInfo(LADigest[:])
	SignedStr, err := SignedInfo.WriteToString()
	if err != nil {
		return "", err
	}
	SignedStr = strings.Replace(SignedStr, "\n", "", -1)
	SignedDigest := sha256.Sum256([]byte(SignedStr))
	r, s, err := ecdsa.Sign(rand.Reader, signingKey.Key, SignedDigest[:])
	if err != nil {
		return "", fmt.Errorf("failed to sign: %v", err)
	}
	sig := r.Bytes()
	sig = append(sig, s.Bytes()...)
	challenge := c.Root(LA, SignedInfo, sig, signingKey.PublicBytes())
	base, err := challenge.WriteToString()
	if err != nil {
		return "", err
	}
	xmlHeader := `<?xml version="1.0" encoding="utf-8"?>`
	challengeStr := xmlHeader + base
	return strings.Replace(challengeStr, "\n", "", -1), nil
}

func (Challenge) Root(LA *etree.Document, SignedInfo *etree.Document, Signature []byte, SigningPublicKey []byte) *etree.Document {
	doc := etree.NewDocument()
	doc.WriteSettings.CanonicalEndTags = true

	doc.CreateChild("soap:Envelope", func(e *etree.Element) {
		e.CreateAttr("xmlns:xsi", "http://www.w3.org/2001/XMLSchema-instance")
		e.CreateAttr("xmlns:xsd", "http://www.w3.org/2001/XMLSchema")
		e.CreateAttr("xmlns:soap", "http://schemas.xmlsoap.org/soap/envelope/")
		e.CreateChild("soap:Body", func(e *etree.Element) {
			e.CreateChild("AcquireLicense", func(e *etree.Element) {
				e.CreateAttr("xmlns", "http://schemas.microsoft.com/DRM/2007/03/protocols")
				e.CreateChild("challenge", func(e *etree.Element) {
					e.CreateChild("Challenge", func(e *etree.Element) {
						e.CreateAttr("xmlns", "http://schemas.microsoft.com/DRM/2007/03/protocols/messages")
						e.AddChild(LA.Root())

						e.CreateChild("Signature", func(e *etree.Element) {
							e.CreateAttr("xmlns", "http://www.w3.org/2000/09/xmldsig#")
							e.AddChild(SignedInfo.Root())
							e.CreateChild("SignatureValue", func(e *etree.Element) {
								e.CreateText(base64.StdEncoding.EncodeToString(Signature))
							})
							e.CreateChild("KeyInfo", func(e *etree.Element) {
								e.CreateAttr("xmlns", "http://www.w3.org/2000/09/xmldsig#")
								e.CreateChild("KeyValue", func(e *etree.Element) {
									e.CreateChild("ECCKeyValue", func(e *etree.Element) {
										e.CreateChild("PublicKey", func(e *etree.Element) {
											e.CreateText(base64.StdEncoding.EncodeToString(SigningPublicKey))
										})
									})
								})
							})
						})
					})
				})
			})
		})
	})
	doc.Indent(0)
	return doc
}
