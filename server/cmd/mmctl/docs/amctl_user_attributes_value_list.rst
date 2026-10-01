.. _amctl_user_attributes_value_list:

amctl user attributes value list
--------------------------------

List User Attributes values for a user

Synopsis
~~~~~~~~


List all User Attributes values for a specific user.

::

  amctl user attributes value list [user] [flags]

Examples
~~~~~~~~

::

    user attributes value list john.doe@company.com
    user attributes value list johndoe

Options
~~~~~~~

::

  -h, --help   help for list

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

* `amctl user attributes value <amctl_user_attributes_value.rst>`_ 	 - Management of User Attributes values

