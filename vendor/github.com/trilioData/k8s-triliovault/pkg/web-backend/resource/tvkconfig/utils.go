package tvkconfig

import (
	"context"
	"crypto/md5" //nolint:gosec // To use md5 hash as a key in a map
	"encoding/hex"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/trilioData/k8s-triliovault/api/v1"
	"github.com/trilioData/k8s-triliovault/internal"
	"github.com/trilioData/k8s-triliovault/pkg/web-backend/errors"
)

func getTVKConfigHolerSecret(k8sClient client.Client) (*corev1.Secret, error) {
	log := ctrl.Log.WithName("function").WithName("getTVKConfigHolerSecret")
	recommendedLabels := internal.GetRecommendedLabels(TVKConfigHolderName, internal.ManagedBy)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TVKConfigHolderName,
			Namespace: internal.GetInstallNamespace(),
			Labels:    recommendedLabels,
		},
		Data: make(map[string][]byte),
	}
	// Get secret if already exists
	err := k8sClient.Get(context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, secret)
	if err != nil && !apierrors.IsNotFound(err) {
		log.Error(err, "error while retrieving secret")
		return secret, err
	} else if err == nil {
		return secret, nil
	}

	// Create new secret if it doesn't exists
	err = k8sClient.Create(context.TODO(), secret)
	if err != nil {
		log.Error(err, "error while creating secret")
		return secret, err
	}
	return secret, nil
}

func GetAuthUserInfo(k8sClient client.Client) (*authenticationv1.UserInfo, error) {

	log := ctrl.Log.WithName("function").WithName("GetAuthUserInfo")
	userInfo := &authenticationv1.UserInfo{}
	// Policy with a label to get auth user info from webhook server
	policy := &v1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PolicyNameForAuthCheck,
			Namespace: internal.GetInstallNamespace(),
			Labels:    map[string]string{internal.CheckAuthInfoLabel: strconv.FormatBool(true)},
		},
	}
	err := k8sClient.Create(context.TODO(), policy)
	if err == nil {
		err = errors.NewInternal("Unable to find auth user info for provided client")
		log.Error(err, "policy got created while retrieving auth info")
		return userInfo, err
	}

	// If policy has a label checkAuthInfo then TVK webhook server will deny create request and send auth user info in format
	// UserName:[username] UserUID:[user-uid] Groups[list of groups] which will be parsed.
	webhookMessage := err.Error()
	r := regexp.MustCompile(`\[(.*?)\]`)
	matches := r.FindAllStringSubmatch(webhookMessage, -1)

	// Validate response message matches
	if len(matches) != 3 {
		err = errors.NewInternal("Unable to parse auth user details from webhook response")
		log.Error(err, "error while parsing webhook response")
		return userInfo, err
	}
	for _, element := range matches {
		if len(element) != 2 {
			err = errors.NewInternal("Unable to parse auth user details from webhook response")
			log.Error(err, "error while parsing webhook response")
			return userInfo, err
		}
	}

	// Assign parsed user info
	userInfo.Username = matches[0][1]
	userInfo.UID = matches[1][1]
	userInfo.Groups = strings.Split(matches[2][1], internal.Comma)
	return userInfo, nil
}

func getHashString(str string) string {
	strBytes := md5.Sum([]byte(str)) //nolint:gosec // To use md5 hash as a key in a map
	return hex.EncodeToString(strBytes[:])
}

func isValidURL(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}
