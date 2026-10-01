.. _amctl_completion_bash:

amctl completion bash
---------------------

Generate the autocompletion script for bash

Synopsis
~~~~~~~~


Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(amctl completion bash)

To load completions for every new session, execute once:

#### Linux:

	amctl completion bash > /etc/bash_completion.d/amctl

#### macOS:

	amctl completion bash > $(brew --prefix)/etc/bash_completion.d/amctl

You will need to start a new shell for this setup to take effect.


::

  amctl completion bash

Options
~~~~~~~

::

  -h, --help              help for bash
      --no-descriptions   disable completion descriptions

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

* `amctl completion <amctl_completion.rst>`_ 	 - Generate the autocompletion script for the specified shell

