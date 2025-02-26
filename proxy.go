package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
)

var customTransport = http.DefaultTransport
var config Config

func main() {

	configFile := flag.String(
		"config", "config.yaml", "path to the config file")

	flag.Parse()

	data, err := os.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("cannot read file: %v", err)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		log.Fatalf("cannot unmarshal data: %v", err)
	}

	if config.TLS.Enabled && config.TLS.CaFile != "" {

		// Load CA cert
		caCert, err := os.ReadFile(config.TLS.CaFile)
		if err != nil {
			log.Fatal(err)
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		// Setup HTTPS client
		tlsConfig := &tls.Config{
			//Certificates: []tls.Certificate{cert},
			RootCAs: caCertPool,
		}
		customTransport = &http.Transport{TLSClientConfig: tlsConfig}
	}

	sr, err := url.Parse(config.Registry)
	if err != nil {
		log.Fatalf("cannot parse registry url: %v", err)
	}

	config.Scheme = sr.Scheme
	config.Host = sr.Host

	// Create a new HTTP server with the handleRequest function as the handler
	server := http.Server{
		Addr:    ":" + config.Port,
		Handler: http.HandlerFunc(handleRequest),
		//TLSConfig: config,
	}

	// Start the server and log any errors
	log.Println("Starting proxy server on " + server.Addr)

	if config.TLS.Enabled {
		err := server.ListenAndServeTLS(config.TLS.ClientCertFile, config.TLS.ClientKeyFile)
		if err != nil {
			log.Fatal("Error starting proxy server: ", err)
		}
	} else {
		err := server.ListenAndServe()
		if err != nil {
			log.Fatal("Error starting proxy server: ", err)
		}
	}

}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	// Create a new HTTP request with the same method, URL, and body as the original request
	targetURL := r.URL

	// Repoint the request to our target server
	targetURL.Scheme = config.Scheme
	targetURL.Host = config.Host

	shouldMaybeTransform := false

	if targetURL.Query().Has("format") {
		format := targetURL.Query().Get("format")
		if format == "serialized" {
			shouldMaybeTransform = true
			// Remove the format parameter from the request since it's not supported at this time
			values := targetURL.Query()
			values.Del("format")
			targetURL.RawQuery = values.Encode()
		} else {
			http.Error(w, fmt.Sprintf("Unknown format: %v", format), http.StatusInternalServerError)
			return
		}
	}

	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
		return
	}

	// Copy the headers from the original request to the proxy request
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Send the proxy request using the custom transport
	resp, err := customTransport.RoundTrip(proxyReq)
	if err != nil {
		http.Error(w, "Error sending proxy request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	// Copy the headers from the proxy response to the original response
	for name, values := range resp.Header {
		for _, value := range values {
			if name != "Content-Length" {
				w.Header().Add(name, value)
			}
		}
	}

	// Set the status code of the original response to the status code of the proxy response
	w.WriteHeader(resp.StatusCode)

	if shouldMaybeTransform {
		// First, unpack the response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "Error reading proxy response", http.StatusInternalServerError)
			return
		}

		schemaResponse := make(map[string]interface{})
		err = json.Unmarshal(body, &schemaResponse)
		if err != nil {
			http.Error(w, "Error unmarshalling proxy response", http.StatusInternalServerError)
			return
		}

		schemaType, schemaTypeOk := schemaResponse["schemaType"]
		schema, schemaOk := schemaResponse["schema"]

		// Next, figure out if the response was for protobuf and there is a schema field
		if schemaTypeOk && schemaType == "PROTOBUF" && schemaOk {
			// It was protobuf, so encode the schema and stuff the encoded result into the response
			schemaResponse["schema"], err = encodeSchema(schema.(string))
			if err != nil {
				http.Error(w, "Error encoding schema: "+err.Error(), http.StatusInternalServerError)
				return
			}
			body, err = json.Marshal(schemaResponse)
			io.Copy(w, bytes.NewReader(body))
			return
		}
	}

	// Copy the body of the proxy response to the original response
	io.Copy(w, resp.Body)
	return

}

func encodeSchema(schema string) (string, error) {
	compiler := protocompile.Compiler{
		Resolver: &protocompile.SourceResolver{},
	}
	f, _ := os.CreateTemp("/tmp", "sample")
	_, err := f.Write([]byte(schema))
	if err != nil {
		return "", errors.New("error creating temp file")
	}
	err = f.Close()
	if err != nil {
		return "", err
	}

	fls, err := compiler.Compile(context.TODO(), f.Name())
	if err != nil {
		return "", errors.New("error compiling schema: " + err.Error())
	}
	fdp := protodesc.ToFileDescriptorProto(fls[0].ParentFile())
	raw, err := proto.Marshal(fdp)
	if err != nil {
		return "", errors.New("error marshalling protobuf file")
	}
	encoded := b64.StdEncoding.EncodeToString(raw)

	err = os.Remove(f.Name())
	if err != nil {
		return "", errors.New("error removing temp file")
	}

	return encoded, nil
}
