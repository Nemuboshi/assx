-- Add this to Neovim 0.11+ configuration. Put assx on PATH.
vim.filetype.add({ extension = { ass = 'ass', ssa = 'ass' } })
vim.lsp.config('assx', {
  cmd = { 'assx', 'lsp' },
  filetypes = { 'ass' },
  workspace_required = false,
  init_options = {
    assx = {
      primaryRenderer = 'libass',
      compatibilityRenderers = { 'xy-vsfilter' },
    },
  },
})
vim.lsp.enable('assx')
