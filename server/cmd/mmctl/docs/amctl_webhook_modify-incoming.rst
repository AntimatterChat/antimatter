.. _amctl_webhook_modify-incoming:

amctl webhook modify-incoming
-----------------------------

Modify incoming webhook

Synopsis
~~~~~~~~


Modify existing incoming webhook by changing its title, description, channel or icon url

::

  amctl webhook modify-incoming [flags]

Examples
~~~~~~~~

::

    webhook modify-incoming [webhookID] --channel [channelID] --display-name [displayName] --description [webhookDescription] --lock-to-channel --icon [iconURL]

Options
~~~~~~~

::

      --channel string        Channel ID
      --description string    Incoming webhook description
      --display-name string   Incoming webhook display name
  -h, --help                  help for modify-incoming
      --icon string           Icon URL
      --lock-to-channel       Lock to channel

Options inherited from parent commands
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

::

      --config string                path to the configuration file (default "$XDG_CONFIG_HOME/amctl/config")
      --disable-pager                disables paged output
      --insecure-sha1-intermediate   allows to use insecure TLS protocols, such as SHA-1
      --insecure-tls-version         allows to use TLS versions 1.0 and 1.1
      --json                         the output format will be in json format
      --local                        allows communicating with the server through a unix socket
      --quiet                        prevent amctl to generate output for the commands
      --strict                       will only run commands if the amctl version matches the server one
      --suppress-warnings            disables printing warning messages

SEE ALSO
~~~~~~~~

* `amctl webhook <amctl_webhook.rst>`_ 	 - Management of webhooks

