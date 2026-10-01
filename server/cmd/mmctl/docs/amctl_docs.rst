.. _amctl_docs:

amctl docs
----------

Generates amctl documentation

Synopsis
~~~~~~~~


Generates amctl documentation

::

  amctl docs [flags]

Options
~~~~~~~

::

  -d, --directory string   The directory where the docs would be generated in. (default "docs")
  -h, --help               help for docs

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

