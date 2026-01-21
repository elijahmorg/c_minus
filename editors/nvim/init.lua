-- C-minus Neovim LSP bootstrap
--
-- This starts the C-minus LSP server for *.cm files.
--
-- Binary path: repo-local build output.
local c_minus_lsp_cmd = { "/Users/elijahmorgan/LocalDocs/projects/c_plus/c_minus_lsp" }

local function find_root(fname)
  local start = vim.fs.dirname(fname)
  local mod = vim.fs.find({ "cm.mod" }, { upward = true, path = start })[1]
  if not mod then
    return nil
  end
  return vim.fs.dirname(mod)
end

-- Optional filetype assignment.
vim.filetype.add({
  extension = {
    cm = "cminus",
  },
})

vim.api.nvim_create_autocmd({ "BufReadPost", "BufNewFile" }, {
  pattern = "*.cm",
  callback = function(args)
    local buf = args.buf
    local fname = vim.api.nvim_buf_get_name(buf)
    if fname == "" then
      return
    end

    local root = find_root(fname)
    if not root then
      return
    end

    vim.lsp.start({
      name = "c_minus_lsp",
      cmd = c_minus_lsp_cmd,
      root_dir = root,
    })
  end,
})
