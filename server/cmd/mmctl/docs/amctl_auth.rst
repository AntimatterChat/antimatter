.. _amctl_auth:

amctl auth
----------

Manages the credentials of the remote Antimatter instances

Synopsis
~~~~~~~~


Manages the credentials of the remote Antimatter instances

Options
~~~~~~~

::

  -h, --help   help for auth

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

* `amctl <amctl.rst>`_ 	 - Remote client for the Open Source, self-hosted Slack-alternative
* `amctl auth clean <amctl_auth_clean.rst>`_ 	 - Clean all credentials
* `amctl auth current <amctl_auth_current.rst>`_ 	 - Show current user credentials
* `amctl auth delete <amctl_auth_delete.rst>`_ 	 - Delete an credentials
* `amctl auth list <amctl_auth_list.rst>`_ 	 - Lists the credentials
* `amctl auth login <amctl_auth_login.rst>`_ 	 - Login into an instance
* `amctl auth renew <amctl_auth_renew.rst>`_ 	 - Renews a set of credentials
* `amctl auth set <amctl_auth_set.rst>`_ 	 - Set the credentials to use

