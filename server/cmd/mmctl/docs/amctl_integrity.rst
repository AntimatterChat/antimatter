.. _amctl_integrity:

amctl integrity
---------------

Check database records integrity.

Synopsis
~~~~~~~~


Perform a relational integrity check which returns information about any orphaned record found.

::

  amctl integrity [flags]

Options
~~~~~~~

::

      --confirm   Confirm you really want to run a complete integrity check that may temporarily harm system performance
  -h, --help      help for integrity
  -v, --verbose   Show detailed information on integrity check results

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

