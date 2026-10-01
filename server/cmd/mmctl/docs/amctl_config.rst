.. _amctl_config:

amctl config
------------

Configuration

Synopsis
~~~~~~~~


Configuration

Options
~~~~~~~

::

  -h, --help   help for config

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
* `amctl config edit <amctl_config_edit.rst>`_ 	 - Edit the config
* `amctl config export <amctl_config_export.rst>`_ 	 - Export the server configuration
* `amctl config get <amctl_config_get.rst>`_ 	 - Get config setting
* `amctl config migrate <amctl_config_migrate.rst>`_ 	 - Migrate existing config between backends
* `amctl config patch <amctl_config_patch.rst>`_ 	 - Patch the config
* `amctl config reload <amctl_config_reload.rst>`_ 	 - Reload the server configuration
* `amctl config reset <amctl_config_reset.rst>`_ 	 - Reset config setting
* `amctl config set <amctl_config_set.rst>`_ 	 - Set config setting
* `amctl config show <amctl_config_show.rst>`_ 	 - Writes the server configuration to STDOUT
* `amctl config subpath <amctl_config_subpath.rst>`_ 	 - Update client asset loading to use the configured subpath

