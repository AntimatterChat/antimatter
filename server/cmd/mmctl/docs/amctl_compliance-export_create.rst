.. _amctl_compliance-export_create:

amctl compliance-export create
------------------------------

Create a compliance export job, of type 'csv' or 'actiance' or 'globalrelay'

Synopsis
~~~~~~~~


Create a compliance export job, of type 'csv' or 'actiance' or 'globalrelay'. If --date is set, the job will run for one day, from 12am to 12am (minus one millisecond) inclusively, in the format with timezone offset: `"YYYY-MM-DD -0000"`. E.g., "2024-10-21 -0400" for Oct 21, 2024 EDT timezone. "2023-11-01 +0000" for Nov 01, 2024 UTC. If set, the 'start' and 'end' flags will be ignored.

Important: Running a compliance export job from amctl will NOT affect the next scheduled job's batch_start_time. This means that if you run a compliance export job from amctl, the next scheduled job will run from the batch_end_time of the previous scheduled job, as usual.

::

  amctl compliance-export create [complianceExportType] --date "2025-03-27 -0400" [flags]

Examples
~~~~~~~~

::

  compliance-export create csv --date "2025-03-27 -0400"

Options
~~~~~~~

::

      --date "YYYY-MM-DD -0000"   Run the export for one day, from 12am to 12am (minus one millisecond) inclusively, in the format with timezone offset: "YYYY-MM-DD -0000". E.g., `"2024-10-21 -0400"` for Oct 21, 2024 EDT timezone. `"2023-11-01 +0000"` for Nov 01, 2024 UTC. If set, the 'start' and 'end' flags will be ignored.
      --end 1743134400000         The end timestamp in unix milliseconds. Posts with updateAt <= end will be exported. If set, 'start' must be set as well. eg, 1743134400000 for 2025-03-28 EDT.
  -h, --help                      help for create
      --start 1743048000000       The start timestamp in unix milliseconds. Posts with updateAt >= start will be exported. If set, 'end' must be set as well. eg, 1743048000000 for 2025-03-27 EDT.

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

* `amctl compliance-export <amctl_compliance-export.rst>`_ 	 - Management of compliance exports

