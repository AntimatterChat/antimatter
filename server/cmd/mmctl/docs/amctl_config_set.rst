.. _amctl_config_set:

amctl config set
----------------

Set config setting

Synopsis
~~~~~~~~


Sets the value of a config setting by its name in dot notation. Accepts multiple values for array settings

::

  amctl config set [flags]

Examples
~~~~~~~~

::

  config set SqlSettings.DriverName postgres
  config set SqlSettings.DataSourceReplicas "replica1" "replica2"

Options
~~~~~~~

::

  -h, --help   help for set

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

* `amctl config <amctl_config.rst>`_ 	 - Configuration

