-- C-minus Neovim LSP + Tree-sitter bootstrap

local repo_root = "/Users/elijahmorgan/LocalDocs/projects/c_plus"
local c_minus_lsp_cmd = { repo_root .. "/c_minus_lsp" }

local function find_root(fname)
	local start = vim.fs.dirname(fname)
	local mod = vim.fs.find({ "cm.mod" }, { upward = true, path = start })[1]
	if not mod then
		return nil
	end
	return vim.fs.dirname(mod)
end

vim.filetype.add({
	extension = {
		cm = "cminus",
	},
})

local ok, parsers = pcall(require, "nvim-treesitter.parsers")
if ok then
	parsers.get_parser_configs().c_minus = {
		install_info = {
			url = repo_root,
			files = {
				"treesitter/tree-sitter-cminus/src/parser.c",
				"treesitter/tree-sitter-cminus/src/scanner.c",
			},
		},
		filetype = "cminus",
	}

	vim.treesitter.language.register("c_minus", "cminus")
	vim.opt.runtimepath:append(repo_root .. "/editors/nvim")
end

vim.api.nvim_create_autocmd("FileType", {
	pattern = "cminus",
	callback = function(args)
		if vim.treesitter and vim.treesitter.start then
			pcall(vim.treesitter.start, args.buf, "c_minus")
		end
	end,
})

vim.api.nvim_create_autocmd({ "BufReadPost", "BufNewFile" }, {
	pattern = "*.cm",
	callback = function(args)
		local fname = vim.api.nvim_buf_get_name(args.buf)
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
