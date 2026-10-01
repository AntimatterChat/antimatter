.. _amctl_completion_fish:

amctl completion fish
---------------------

Generate the autocompletion script for fish

Synopsis
~~~~~~~~


Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	amctl completion fish | source

To load completions for every new session, execute once:

	amctl completion fish > ~/.config/fish/completions/amctl.fish

You will need to start a new shell for this setup to take effect.


::

  amctl completion fish [flags]

Options
~~~~~~~

::

  -h, --help              help for fish
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

