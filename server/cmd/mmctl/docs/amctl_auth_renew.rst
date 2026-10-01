.. _amctl_auth_renew:

amctl auth renew
----------------

Renews a set of credentials

Synopsis
~~~~~~~~


Renews the credentials for a given server

::

  amctl auth renew [flags]

Examples
~~~~~~~~

::

    auth renew local-server

Options
~~~~~~~

::

  -t, --access-token-file string   Access token file to be read to use instead of username/password
  -h, --help                       help for renew
  -m, --mfa-token string           MFA token for the credentials
  -f, --password-file string       Password file to be read for the credentials

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

* `amctl auth <amctl_auth.rst>`_ 	 - Manages the credentials of the remote Antimatter instances

