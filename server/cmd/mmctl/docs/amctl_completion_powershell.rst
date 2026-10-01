.. _amctl_completion_powershell:

amctl completion powershell
---------------------------

Generate the autocompletion script for powershell

Synopsis
~~~~~~~~


Generate the autocompletion script for powershell.

To load completions in your current shell session:

	amctl completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.


::

  amctl completion powershell [flags]

Options
~~~~~~~

::

  -h, --help              help for powershell
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

