package pg

import (
	"fmt"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/secret"
)

func encryptApplicationSecrets(s secret.Store, app *model.Application) error {
	if app == nil {
		return nil
	}
	var err error
	app.WebhookSecret, err = secret.EncryptOptional(s, app.WebhookSecret)
	if err != nil {
		return fmt.Errorf("encrypt webhook_secret: %w", err)
	}
	app.Secrets, err = secret.EncryptStringMap(s, app.Secrets)
	if err != nil {
		return fmt.Errorf("encrypt secrets: %w", err)
	}
	return nil
}

func decryptApplicationSecrets(s secret.Store, app *model.Application) error {
	if app == nil {
		return nil
	}
	var err error
	app.WebhookSecret, err = secret.DecryptOptional(s, app.WebhookSecret)
	if err != nil {
		return fmt.Errorf("decrypt webhook_secret: %w", err)
	}
	app.Secrets, err = secret.DecryptStringMap(s, app.Secrets)
	if err != nil {
		return fmt.Errorf("decrypt secrets: %w", err)
	}
	return nil
}

func decryptApplications(s secret.Store, apps []model.Application) error {
	for i := range apps {
		if err := decryptApplicationSecrets(s, &apps[i]); err != nil {
			return err
		}
	}
	return nil
}

func encryptUser2FA(s secret.Store, secretVal string) (string, error) {
	return secret.EncryptOptional(s, secretVal)
}

func decryptUser(user *model.User, s secret.Store) error {
	if user == nil {
		return nil
	}
	var err error
	user.TwoFASecret, err = secret.DecryptOptional(s, user.TwoFASecret)
	if err != nil {
		return fmt.Errorf("decrypt two_fa_secret: %w", err)
	}
	return nil
}

func decryptUsers(users []model.User, s secret.Store) error {
	for i := range users {
		if err := decryptUser(&users[i], s); err != nil {
			return err
		}
	}
	return nil
}

func encryptPageWebhook(s secret.Store, page *model.Page) error {
	if page == nil {
		return nil
	}
	var err error
	page.WebhookSecret, err = secret.EncryptOptional(s, page.WebhookSecret)
	if err != nil {
		return fmt.Errorf("encrypt webhook_secret: %w", err)
	}
	return nil
}

func decryptPage(page *model.Page, s secret.Store) error {
	if page == nil {
		return nil
	}
	var err error
	page.WebhookSecret, err = secret.DecryptOptional(s, page.WebhookSecret)
	if err != nil {
		return fmt.Errorf("decrypt webhook_secret: %w", err)
	}
	return nil
}

func decryptPages(pages []model.Page, s secret.Store) error {
	for i := range pages {
		if err := decryptPage(&pages[i], s); err != nil {
			return err
		}
	}
	return nil
}

func encryptWorkerWebhook(s secret.Store, worker *model.Worker) error {
	if worker == nil {
		return nil
	}
	var err error
	worker.WebhookSecret, err = secret.EncryptOptional(s, worker.WebhookSecret)
	if err != nil {
		return fmt.Errorf("encrypt webhook_secret: %w", err)
	}
	return nil
}

func decryptWorker(worker *model.Worker, s secret.Store) error {
	if worker == nil {
		return nil
	}
	var err error
	worker.WebhookSecret, err = secret.DecryptOptional(s, worker.WebhookSecret)
	if err != nil {
		return fmt.Errorf("decrypt webhook_secret: %w", err)
	}
	return nil
}

func decryptWorkers(workers []model.Worker, s secret.Store) error {
	for i := range workers {
		if err := decryptWorker(&workers[i], s); err != nil {
			return err
		}
	}
	return nil
}

func encryptResourceConfig(s secret.Store, cfg []byte) ([]byte, error) {
	return secret.EncryptJSONFieldValues(s, cfg, secret.IsResourceConfigKey)
}

func decryptResourceConfig(s secret.Store, cfg []byte) ([]byte, error) {
	return secret.DecryptJSONFieldValues(s, cfg, secret.IsResourceConfigKey)
}

func encryptNotificationConfig(s secret.Store, cfg []byte) ([]byte, error) {
	return secret.EncryptJSONFieldValues(s, cfg, secret.IsNotificationConfigKey)
}

func decryptNotificationConfig(s secret.Store, cfg []byte) ([]byte, error) {
	return secret.DecryptJSONFieldValues(s, cfg, secret.IsNotificationConfigKey)
}

func encryptNodePassword(s secret.Store, password string) (string, error) {
	return secret.EncryptOptional(s, password)
}

func decryptNode(node *model.ServerNode, s secret.Store) error {
	if node == nil {
		return nil
	}
	var err error
	node.Password, err = secret.DecryptOptional(s, node.Password)
	if err != nil {
		return fmt.Errorf("decrypt password: %w", err)
	}
	return nil
}

func decryptNodes(nodes []model.ServerNode, s secret.Store) error {
	for i := range nodes {
		if err := decryptNode(&nodes[i], s); err != nil {
			return err
		}
	}
	return nil
}
