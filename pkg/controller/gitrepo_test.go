package clusterimageset

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGetGitRepoAuthFromSecret(t *testing.T) {
	c := initClient()

	secret1 := getSecret("secret1", []byte("user1"), []byte("token1"), []byte("key1"), []byte("cert1"))
	_ = c.Create(context.TODO(), secret1)

	type ctrlFields struct {
		client       client.Client
		restMapper   meta.RESTMapper
		log          logr.Logger
		interval     int
		configMap    string
		secret       string
		lastCommitID string
	}
	f1 := ctrlFields{
		client: c,
	}
	f2 := ctrlFields{
		client: c,
		secret: "secret1",
	}

	tests := []struct {
		name             string
		controllerFields ctrlFields
		wantUsername     string
		wantAccessToken  string
		wantClientKey    []byte
		wantClientCert   []byte
		wantErr          bool
	}{
		{
			name:             "no secret",
			controllerFields: f1,
			wantUsername:     "",
			wantAccessToken:  "",
			wantClientKey:    []byte(""),
			wantClientCert:   []byte(""),
			wantErr:          false,
		},
		{
			name:             "has secret",
			controllerFields: f2,
			wantUsername:     "user1",
			wantAccessToken:  "token1",
			wantClientKey:    []byte("key1"),
			wantClientCert:   []byte("cert1"),
			wantErr:          false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ClusterImageSetController{
				client:       tt.controllerFields.client,
				restMapper:   tt.controllerFields.restMapper,
				log:          tt.controllerFields.log,
				interval:     tt.controllerFields.interval,
				configMap:    tt.controllerFields.configMap,
				secret:       tt.controllerFields.secret,
				lastCommitID: tt.controllerFields.lastCommitID,
			}
			gotUsername, gotAccessToken, gotClientKey, gotClientCert, err := r.getGitRepoAuthFromSecret()
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterImageSetController.getGitRepoAuthFromSecret() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotUsername != tt.wantUsername {
				t.Errorf("ClusterImageSetController.getGitRepoAuthFromSecret() got = %v, want %v", gotUsername, tt.wantUsername)
			}
			if gotAccessToken != tt.wantAccessToken {
				t.Errorf("ClusterImageSetController.getGitRepoAuthFromSecret() got1 = %v, want %v", gotAccessToken, tt.wantAccessToken)
			}
			if !reflect.DeepEqual(gotClientKey, tt.wantClientKey) {
				t.Errorf("ClusterImageSetController.getGitRepoAuthFromSecret() got2 = %v, want %v", gotClientKey, tt.wantClientKey)
			}
			if !reflect.DeepEqual(gotClientCert, tt.wantClientCert) {
				t.Errorf("ClusterImageSetController.getGitRepoAuthFromSecret() got3 = %v, want %v", gotClientCert, tt.wantClientCert)
			}
		})
	}
}

func TestGetCertChain(t *testing.T) {
	/*
			cert := `-----BEGIN CERTIFICATE-----
		    MIIFTDCCAzQCCQDUHR2zBw+sDDANBgkqhkiG9w0BAQsFADBoMQswCQYDVQQGEwJV
		    UzELMAkGA1UECAwCTkMxEDAOBgNVBAcMB1JhbGVpZ2gxDzANBgNVBAoMBlJlZEhh
		    dDEOMAwGA1UECwwFUkhBQ00xGTAXBgNVBAMMEGdvZ3Mtc3ZjLWRlZmF1bHQwHhcN
		    MjIxMTAxMjI0ODU3WhcNMjUwODIxMjI0ODU3WjBoMQswCQYDVQQGEwJVUzELMAkG
		    A1UECAwCTkMxEDAOBgNVBAcMB1JhbGVpZ2gxDzANBgNVBAoMBlJlZEhhdDEOMAwG
		    A1UECwwFUkhBQ00xGTAXBgNVBAMMEGdvZ3Mtc3ZjLWRlZmF1bHQwggIiMA0GCSqG
		    SIb3DQEBAQUAA4ICDwAwggIKAoICAQDO6YKrveBcH5sXMpuqQ+AJIhhGbygKIHZ1
		    iCIev8LT0h+gKBVtGjgMcXzgFmCrDU6TXdc96PudZMANB2POHGMu2wrHdH4bgAqN
		    hFkBtKFoDbtnuw+tC0X0UphZUJ4aowJzzrA6yKHjIIPkGErcFAZipTRklYknqzbU
		    80pndIX9dPBswdlCBq6sTuvDrJwL/dRvm8gGQEV6wKzNPfvVEG6/CFYaYp52a2Tx
		    rbDdZh4FYqxjSbTIzOkVhIjelucFDPj4O9wWdVvyhpvqDFFCOqI//qt4dpiyhIQx
		    9iROHNlM4zcMHrzhhiKW7AqVh+rSzqT8DNdzhxloUr3qsXEBYyTtYQE/EMqXMq6h
		    2z2t5SuOSx2CddjJ+eNQ27QE6N4agTK97mlXTUHIr7JfOVXocPqafb6R66GWDxlP
		    eytX7qgBCVzPm7q+OP47D6lGMfWmR6Q/zYAreQu8lULw54+ajZg3EN3cWIUd7jBg
		    9pfvAMBodfmaL4NNDKoHZVl4NH89rwhHmxcAXZjwYVXDRNy4mF6LklbJ76ucafhK
		    C/E3CMLSgrWPV0rgDuaXgfFZRe0nSr59uiJqnsXwpbEO8jdnTcIsqwxUbDdBGd4Z
		    +GW/oeUyVsp0m2Z3CC9csT8US3vyKgohuJ/9kQg6bb4yEgD2vim9JLkBr2WD+7Zs
		    a9v+GT7KsQIDAQABMA0GCSqGSIb3DQEBCwUAA4ICAQBIJtFyVZGGpGs0NC9gUAhQ
		    sO/vIkH1K++ZLqVH5/jnkTeZi7Mn2RfTP8GmkXM8na/sCDj/ZiS0uHh8PEI3scok
		    1kG1B1WntdRUf0mb5/IHtGRcdN7tznVb9sLzVvia8dxfQHUH8n0PtZ3FRW36hT8O
		    e+ULZxsxh/uqqjCLheVfhAmfEbFn60Il8wtNkCX1EMlT1WHU3bBS4juxaGxbuuJq
		    uLf6JNzvrC6cOuvDTECZ7rHwcTL2yKYfGF3exDjgNKD9fYcUFD9Km+kQTKL/BRU2
		    BS70lyPjizn8K3HhcyvKW57QsoahFPoMg0+tYiEn3D8AO8sEbOKdT/lEhXIl293H
		    WlPFiK5v04BckToAAEQotCK/sasFvXIGwI4uu3X73qewri3LtnUbAucpvJRpZqFT
		    uD/mNKgoQL+7FAf708Fqd5fbrD+mpgrI/E8IWwonNEmKt3FWY/kOx6znSf/VYcd0
		    2vGUXSJgMIBCpB7s3h0uCHA9niDMA6S+yGF6JD/BpWJlY6ue/Jd/k6VKjL6uyjRt
		    aYeVhBh/yzAAmPE6/FZnUdgRGSnkm/B8XI1VujpRTYsiKCGq1XpoWwve2Ujlousj
		    vB4YZTsCx9WLCBLqrUQLmYz8OlB2FNAudUwn38C7hyqp0KSU6eKw4cJcljqpxEP2
		    AXDDYhRiaIJMdgKh37ewhw==
		    -----END CERTIFICATE-----`
	*/

	tests := []struct {
		name     string
		certs    string
		wantCert int
	}{
		{
			name:     "no cert",
			certs:    "",
			wantCert: 0,
		},
		/*
			{
				name:     "has cert",
				certs:    cert,
				wantCert: 1,
			}, */
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("cert: %v", tt.certs)
			gotCert := getCertChain(tt.certs)
			if len(gotCert.Certificate) != tt.wantCert {
				t.Errorf("getCertChain() = %v, want %v", len(gotCert.Certificate), tt.wantCert)
			}
		})
	}
}

func TestGetHTTPOptions(t *testing.T) {
	c := initClient()

	configMapInsecureSkipVerify := getDefaultConfigMap()
	configMapInsecureSkipVerify.Data[InsecureSkipVerify] = "true"
	configMapInsecureSkipVerify.Name = "configmap-skip-verify"
	_ = c.Create(context.TODO(), configMapInsecureSkipVerify)

	configMapCaCert := getDefaultConfigMap()
	configMapCaCert.Data[CaCerts] = `-----BEGIN CERTIFICATE-----
    MIIFTDCCAzQCCQDUHR2zBw+sDDANBgkqhkiG9w0BAQsFADBoMQswCQYDVQQGEwJV
    UzELMAkGA1UECAwCTkMxEDAOBgNVBAcMB1JhbGVpZ2gxDzANBgNVBAoMBlJlZEhh
    dDEOMAwGA1UECwwFUkhBQ00xGTAXBgNVBAMMEGdvZ3Mtc3ZjLWRlZmF1bHQwHhcN
    MjIxMTAxMjI0ODU3WhcNMjUwODIxMjI0ODU3WjBoMQswCQYDVQQGEwJVUzELMAkG
    A1UECAwCTkMxEDAOBgNVBAcMB1JhbGVpZ2gxDzANBgNVBAoMBlJlZEhhdDEOMAwG
    A1UECwwFUkhBQ00xGTAXBgNVBAMMEGdvZ3Mtc3ZjLWRlZmF1bHQwggIiMA0GCSqG
    SIb3DQEBAQUAA4ICDwAwggIKAoICAQDO6YKrveBcH5sXMpuqQ+AJIhhGbygKIHZ1
    iCIev8LT0h+gKBVtGjgMcXzgFmCrDU6TXdc96PudZMANB2POHGMu2wrHdH4bgAqN
    hFkBtKFoDbtnuw+tC0X0UphZUJ4aowJzzrA6yKHjIIPkGErcFAZipTRklYknqzbU
    80pndIX9dPBswdlCBq6sTuvDrJwL/dRvm8gGQEV6wKzNPfvVEG6/CFYaYp52a2Tx
    rbDdZh4FYqxjSbTIzOkVhIjelucFDPj4O9wWdVvyhpvqDFFCOqI//qt4dpiyhIQx
    9iROHNlM4zcMHrzhhiKW7AqVh+rSzqT8DNdzhxloUr3qsXEBYyTtYQE/EMqXMq6h
    2z2t5SuOSx2CddjJ+eNQ27QE6N4agTK97mlXTUHIr7JfOVXocPqafb6R66GWDxlP
    eytX7qgBCVzPm7q+OP47D6lGMfWmR6Q/zYAreQu8lULw54+ajZg3EN3cWIUd7jBg
    9pfvAMBodfmaL4NNDKoHZVl4NH89rwhHmxcAXZjwYVXDRNy4mF6LklbJ76ucafhK
    C/E3CMLSgrWPV0rgDuaXgfFZRe0nSr59uiJqnsXwpbEO8jdnTcIsqwxUbDdBGd4Z
    +GW/oeUyVsp0m2Z3CC9csT8US3vyKgohuJ/9kQg6bb4yEgD2vim9JLkBr2WD+7Zs
    a9v+GT7KsQIDAQABMA0GCSqGSIb3DQEBCwUAA4ICAQBIJtFyVZGGpGs0NC9gUAhQ
    sO/vIkH1K++ZLqVH5/jnkTeZi7Mn2RfTP8GmkXM8na/sCDj/ZiS0uHh8PEI3scok
    1kG1B1WntdRUf0mb5/IHtGRcdN7tznVb9sLzVvia8dxfQHUH8n0PtZ3FRW36hT8O
    e+ULZxsxh/uqqjCLheVfhAmfEbFn60Il8wtNkCX1EMlT1WHU3bBS4juxaGxbuuJq
    uLf6JNzvrC6cOuvDTECZ7rHwcTL2yKYfGF3exDjgNKD9fYcUFD9Km+kQTKL/BRU2
    BS70lyPjizn8K3HhcyvKW57QsoahFPoMg0+tYiEn3D8AO8sEbOKdT/lEhXIl293H
    WlPFiK5v04BckToAAEQotCK/sasFvXIGwI4uu3X73qewri3LtnUbAucpvJRpZqFT
    uD/mNKgoQL+7FAf708Fqd5fbrD+mpgrI/E8IWwonNEmKt3FWY/kOx6znSf/VYcd0
    2vGUXSJgMIBCpB7s3h0uCHA9niDMA6S+yGF6JD/BpWJlY6ue/Jd/k6VKjL6uyjRt
    aYeVhBh/yzAAmPE6/FZnUdgRGSnkm/B8XI1VujpRTYsiKCGq1XpoWwve2Ujlousj
    vB4YZTsCx9WLCBLqrUQLmYz8OlB2FNAudUwn38C7hyqp0KSU6eKw4cJcljqpxEP2
    AXDDYhRiaIJMdgKh37ewhw==
    -----END CERTIFICATE-----`
	configMapCaCert.Name = "configmap-cacert"
	_ = c.Create(context.TODO(), configMapCaCert)

	secretUser := getSecret("secretUser", []byte("user1"), []byte("token1"), []byte(""), []byte(""))
	_ = c.Create(context.TODO(), secretUser)

	secretCert := getSecret("secretCert", []byte(""), []byte(""), []byte("key1"), []byte("cert1"))
	_ = c.Create(context.TODO(), secretCert)

	zapLog, _ := zap.NewDevelopment()

	type ctrlFields struct {
		client       client.Client
		restMapper   meta.RESTMapper
		log          logr.Logger
		interval     int
		configMap    string
		secret       string
		lastCommitID string
	}

	f1 := ctrlFields{
		client:       c,
		log:          zapr.NewLogger(zapLog),
		configMap:    "configmap-skip-verify",
		secret:       "secretUser",
		lastCommitID: "abc",
	}
	f2 := ctrlFields{
		client:    c,
		log:       zapr.NewLogger(zapLog),
		configMap: "configmap-cacert",
		secret:    "secretCert",
	}
	tests := []struct {
		name             string
		controllerFields ctrlFields
		wantErr          bool
	}{
		{
			name:             "insecureSkipVerify and user auth",
			controllerFields: f1,
			wantErr:          false,
		},
		{
			name:             "key and cert auth",
			controllerFields: f2,
			wantErr:          true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ClusterImageSetController{
				client:       tt.controllerFields.client,
				restMapper:   tt.controllerFields.restMapper,
				log:          tt.controllerFields.log,
				interval:     tt.controllerFields.interval,
				configMap:    tt.controllerFields.configMap,
				secret:       tt.controllerFields.secret,
				lastCommitID: tt.controllerFields.lastCommitID,
			}
			_, err := r.getHTTPOptions()
			if (err != nil) != tt.wantErr {
				t.Errorf("ClusterImageSetController.getHTTPOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}
