// Command seed populates the demo: two citizens, three documents each, issued
// through the real REST API so the seeded state is indistinguishable from state
// a live demo produces. It also writes every PDF to a folder, plus one tampered
// copy, so there is something to drag into the verify screen on stage.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type doc struct {
	citizen  string
	dept     string
	docType  string
	docID    string
	title    string
	lines    []string
	filename string
}

var seedDocs = []doc{
	{
		// Issued with the name misspelled. The Birth Dept corrects it further down
		// by superseding this record rather than editing it.
		citizen: "Asha Menon", dept: "birth", docType: "birth_certificate", docID: "BC-2019-004471",
		title: "CERTIFICATE OF BIRTH",
		lines: []string{"Name: Aasha Menonn", "Date of Birth: 14 March 1999", "Place: Panaji, Goa",
			"Registration No: BC-2019-004471", "Issued by: Birth Registration Dept"},
		filename: "asha-menon-birth-certificate-v1.pdf",
	},
	{
		citizen: "Asha Menon", dept: "transport", docType: "driving_licence", docID: "DL-GA-2021-88120",
		title: "DRIVING LICENCE",
		lines: []string{"Name: Asha Menon", "Licence No: DL-GA-2021-88120", "Class: LMV",
			"Valid Until: 30 June 2041", "Issued by: Transport Dept"},
		filename: "asha-menon-driving-licence.pdf",
	},
	{
		citizen: "Asha Menon", dept: "education", docType: "degree_certificate", docID: "DEG-2021-1174",
		title: "BACHELOR OF ENGINEERING",
		lines: []string{"Name: Asha Menon", "Degree: B.E. Computer Engineering", "Class: First Class with Distinction",
			"Certificate No: DEG-2021-1174", "Issued by: Education Dept"},
		filename: "asha-menon-degree.pdf",
	},
	{
		citizen: "Rahul Iyer", dept: "birth", docType: "birth_certificate", docID: "BC-1997-000912",
		title: "CERTIFICATE OF BIRTH",
		lines: []string{"Name: Rahul Iyer", "Date of Birth: 2 September 1997", "Place: Margao, Goa",
			"Registration No: BC-1997-000912", "Issued by: Birth Registration Dept"},
		filename: "rahul-iyer-birth-certificate.pdf",
	},
	{
		citizen: "Rahul Iyer", dept: "transport", docType: "driving_licence", docID: "DL-GA-2016-33017",
		title: "DRIVING LICENCE",
		lines: []string{"Name: Rahul Iyer", "Licence No: DL-GA-2016-33017", "Class: LMV, MCWG",
			"Valid Until: 12 January 2036", "Issued by: Transport Dept"},
		filename: "rahul-iyer-driving-licence.pdf",
	},
	{
		citizen: "Rahul Iyer", dept: "education", docType: "degree_certificate", docID: "DEG-2019-0455",
		title: "MASTER OF COMMERCE",
		lines: []string{"Name: Rahul Iyer", "Degree: M.Com", "Class: First Class",
			"Certificate No: DEG-2019-0455", "Issued by: Education Dept"},
		filename: "rahul-iyer-degree.pdf",
	},
}

// The licence that gets revoked on stage, to show a REVOKED verdict on a file
// whose bytes are perfectly genuine.
const revokedDocID = "DL-GA-2016-33017"

// The birth certificate that gets corrected on stage. v1 keeps its place in the
// record as SUPERSEDED — a correction adds history instead of erasing it.
const supersededDocID = "BC-2019-004471"
const correctedDocID = "BC-2019-004471-R1"

func main() {
	apiURL := flag.String("api", "http://127.0.0.1:8088", "backend base URL")
	outDir := flag.String("out", "demo-files", "where to write the seeded PDFs")
	flag.Parse()

	if err := waitForAPI(*apiURL, 30*time.Second); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatal(err)
	}

	var revokeHash, revokeDept string
	var supersedeHash string

	for _, d := range seedDocs {
		pdf := renderPDF(d.title, d.lines, d.docID)

		path := filepath.Join(*outDir, d.filename)
		if err := os.WriteFile(path, pdf, 0o644); err != nil {
			log.Fatal(err)
		}

		hash, err := issue(*apiURL, d, pdf)
		if err != nil {
			log.Fatalf("issuing %s: %v", d.docID, err)
		}
		log.Printf("issued  %-18s %-22s %s  %s", d.docType, d.docID, d.citizen, hash[:18]+"...")

		if d.docID == revokedDocID {
			revokeHash, revokeDept = hash, d.dept
		}
		if d.docID == supersededDocID {
			supersedeHash = hash
		}
	}

	// The correction: same citizen, same document, spelled right. Superseding
	// leaves v1 on chain as SUPERSEDED and points its id at this one.
	corrected := renderPDF("CERTIFICATE OF BIRTH", []string{
		"Name: Asha Menon", "Date of Birth: 14 March 1999", "Place: Panaji, Goa",
		"Registration No: BC-2019-004471-R1 (supersedes BC-2019-004471)",
		"Issued by: Birth Registration Dept",
	}, correctedDocID)
	correctedPath := filepath.Join(*outDir, "asha-menon-birth-certificate-v2.pdf")
	if err := os.WriteFile(correctedPath, corrected, 0o644); err != nil {
		log.Fatal(err)
	}
	if supersedeHash == "" {
		log.Fatalf("never captured the hash of %s, cannot supersede it", supersededDocID)
	}
	if err := supersede(*apiURL, supersedeHash, correctedDocID, "birth", "Asha Menon", corrected); err != nil {
		log.Fatalf("superseding %s: %v", supersededDocID, err)
	}
	log.Printf("superseded %-17s %s -> %s", "birth_certificate", supersededDocID, correctedDocID)

	// A tampered copy of Rahul's degree: the class is upgraded, but the docId
	// marker is left untouched, so the registry still recognises the document and
	// can show that these bytes are not the bytes it anchored.
	tampered := renderPDF("MASTER OF COMMERCE", []string{
		"Name: Rahul Iyer", "Degree: M.Com", "Class: First Class with Distinction",
		"Certificate No: DEG-2019-0455", "Issued by: Education Dept",
	}, "DEG-2019-0455")
	tamperedPath := filepath.Join(*outDir, "rahul-iyer-degree-TAMPERED.pdf")
	if err := os.WriteFile(tamperedPath, tampered, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote   tampered copy      %s", tamperedPath)

	// A forgery that was never issued at all: it carries a marker, but for a
	// docId the registry has never heard of.
	forged := renderPDF("DRIVING LICENCE", []string{
		"Name: Mallory Fernandes", "Licence No: DL-GA-2024-99999", "Class: LMV, MCWG, TRANS",
		"Valid Until: 1 January 2099", "Issued by: Transport Dept",
	}, "DL-GA-2024-99999")
	forgedPath := filepath.Join(*outDir, "never-issued-driving-licence.pdf")
	if err := os.WriteFile(forgedPath, forged, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote   never-issued copy  %s", forgedPath)

	// And revoke one genuine licence.
	if revokeHash != "" {
		if err := revoke(*apiURL, revokeHash, revokeDept); err != nil {
			log.Fatalf("revoking %s: %v", revokedDocID, err)
		}
		log.Printf("revoked %-18s %s", "driving_licence", revokedDocID)
	}

	fmt.Println()
	fmt.Println("demo files in ./" + *outDir + ":")
	fmt.Println("  asha-menon-birth-certificate-v2.pdf  -> VALID")
	fmt.Println("  asha-menon-birth-certificate-v1.pdf  -> SUPERSEDED (points at v2)")
	fmt.Println("  rahul-iyer-driving-licence.pdf       -> REVOKED")
	fmt.Println("  rahul-iyer-degree-TAMPERED.pdf       -> TAMPERED")
	fmt.Println("  never-issued-driving-licence.pdf     -> NOT_ISSUED")
}

func waitForAPI(base string, limit time.Duration) error {
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("backend at %s never became healthy", base)
}

// issue posts one document and returns its on-chain hash.
func issue(base string, d doc, pdf []byte) (string, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	part, err := mw.CreateFormFile("file", d.filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(pdf); err != nil {
		return "", err
	}
	for k, v := range map[string]string{
		"dept": d.dept, "doc_type": d.docType, "doc_id": d.docID, "citizen": d.citizen,
	} {
		if err := mw.WriteField(k, v); err != nil {
			return "", err
		}
	}
	if err := mw.Close(); err != nil {
		return "", err
	}

	resp, err := http.Post(base+"/issue", mw.FormDataContentType(), &body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("%s: %s", resp.Status, raw)
	}
	var out struct {
		DocHash string `json:"docHash"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	return out.DocHash, nil
}

// supersede uploads the corrected document and links it to the one it replaces.
func supersede(base, oldHash, newDocID, dept, citizen string, pdf []byte) error {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	part, err := mw.CreateFormFile("file", newDocID+".pdf")
	if err != nil {
		return err
	}
	if _, err := part.Write(pdf); err != nil {
		return err
	}
	for k, v := range map[string]string{
		"dept": dept, "doc_id": newDocID, "citizen": citizen, "old_hash": oldHash,
	} {
		if err := mw.WriteField(k, v); err != nil {
			return err
		}
	}
	if err := mw.Close(); err != nil {
		return err
	}

	resp, err := http.Post(base+"/supersede", mw.FormDataContentType(), &body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, raw)
	}
	return nil
}

func revoke(base, docHash, dept string) error {
	body, _ := json.Marshal(map[string]string{"docHash": docHash, "dept": dept})
	resp, err := http.Post(base+"/revoke", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, raw)
	}
	return nil
}
