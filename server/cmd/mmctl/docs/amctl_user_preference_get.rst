.. _amctl_user_preference_get:

amctl user preference get
-------------------------

Get a specific user preference

Synopsis
~~~~~~~~


Get a specific user preference

::

  amctl user preference get --category [category] --name [name] [users] [flags]

Examples
~~~~~~~~

::

  preference get --category display_settings --name use_military_time user@example.com

Options
~~~~~~~

::

  -c, --category string   The category of the preference
  -h, --help              help for get
  -n, --name string       The name of the preference

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

* `amctl user preference <amctl_user_preference.rst>`_ 	 - Manage user preferences

