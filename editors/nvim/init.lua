-- C-minus Neovim LSP bootstrap
--
-- This starts the C-minus LSP server for *.cm files.
--
-- Binary path: repo-local build output.
local c_minus_lsp_cmd = { "/Users/elijahmorgan/LocalDocs/projects/c_plus/c_minus_lsp" }

local function resolve_repo_root()
	local candidates = {}

	if vim.env.C_MINUS_REPO and vim.env.C_MINUS_REPO ~= "" then
		table.insert(candidates, vim.env.C_MINUS_REPO)
	end

	local lsp_path = c_minus_lsp_cmd[1]
	if lsp_path and lsp_path ~= "" then
		local resolved = vim.fn.exepath(lsp_path)
		if resolved ~= "" then
			lsp_path = resolved
		end
		table.insert(candidates, vim.fs.dirname(lsp_path))
	end

	local source = debug.getinfo(1, "S").source
	if type(source) == "string" and source:sub(1, 1) == "@" then
		local script_dir = vim.fs.dirname(source:sub(2))
		if script_dir and script_dir ~= "" then
			local root = vim.fs.dirname(vim.fs.dirname(script_dir))
			if root and root ~= "" then
				table.insert(candidates, root)
			end
		end
	end

	if vim.fn.getcwd() ~= "" then
		table.insert(candidates, vim.fn.getcwd())
	end

	for _, candidate in ipairs(candidates) do
		if candidate and candidate ~= "" then
			local parser_path = candidate .. "/treesitter/tree-sitter-cminus/src/parser.c"
			if vim.loop.fs_stat(parser_path) then
				return candidate
			end
		end
	end

	return nil
end

local repo_root = resolve_repo_root()

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

-- Optional Tree-sitter integration for C-minus.
local function setup_cminus_treesitter()
	if not repo_root or repo_root == "" then
		return
	end

	local treesitter_ok, parsers = pcall(require, "nvim-treesitter.parsers")
	if not treesitter_ok then
		return
	end

	local runtime_path = vim.opt.runtimepath:get()
	local queries_root = repo_root .. "/editors/nvim"
	if not vim.tbl_contains(runtime_path, queries_root) then
		vim.opt.runtimepath:append(queries_root)
	end

	local parser_config = parsers.get_parser_configs()
	parser_config.c_minus = {
		install_info = {
			url = repo_root,
			files = {
				"treesitter/tree-sitter-cminus/src/parser.c",
				"treesitter/tree-sitter-cminus/src/scanner.c",
			},
		},
		filetype = "cminus",
	}

	if vim.treesitter and vim.treesitter.language and vim.treesitter.language.register then
		vim.treesitter.language.register("c_minus", "cminus")
	end

	if vim.treesitter and vim.treesitter.query and vim.treesitter.query.set then
		local highlights_path = repo_root .. "/treesitter/tree-sitter-cminus/queries/highlights.scm"
		local ok, lines = pcall(vim.fn.readfile, highlights_path)
		if ok then
			local query_text = table.concat(lines, "\n")
			vim.treesitter.query.set("c_minus", "highlights", query_text)
			vim.treesitter.query.set("cminus", "highlights", query_text)
		end
	end
end

setup_cminus_treesitter()

local function attach_cminus_treesitter(buf)
	setup_cminus_treesitter()
	if not vim.treesitter then
		return
	end

	if vim.treesitter.highlighter
		and vim.treesitter.highlighter.active
		and vim.treesitter.highlighter.active[buf] then
		return
	end

	if vim.treesitter.start then
		local ok = pcall(vim.treesitter.start, buf)
		if ok then
			return
		end
		pcall(vim.treesitter.start, buf, "c_minus")
		return
	end

	local ok, parser = pcall(vim.treesitter.get_parser, buf, "c_minus")
	if not ok or not parser then
		return
	end

	if vim.treesitter.highlighter and vim.treesitter.highlighter.new then
		vim.treesitter.highlighter.new(parser)
	end
end

vim.api.nvim_create_autocmd("FileType", {
	pattern = "cminus",
	callback = function(args)
		attach_cminus_treesitter(args.buf)
	end,
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

		attach_cminus_treesitter(buf)

		vim.lsp.start({
			name = "c_minus_lsp",
			cmd = c_minus_lsp_cmd,
			root_dir = root,
		})
	end,
})
