"""Read-only Python AST extraction. Project sources arrive on stdin, never as imports."""
import ast
import base64
import builtins
import hashlib
import io
import json
import posixpath
import sys
import tokenize

if sys.version_info < (3, 9):
    raise SystemExit("Python 3.9 or newer is required for Python analysis")

payload = json.load(sys.stdin)
result = {"Nodes": [], "Edges": [], "Files": [], "Diagnostics": []}
nodes, edges, modules, symbols = {}, {}, {}, {}
parsed, functions = [], []
uncertain_symbols = set()
root_id = None


def uid(kind, key):
    return kind + "_" + hashlib.sha256(key.encode()).hexdigest()[:24]


def source(path, node):
    return {"fileId": uid("file", path), "startLine": node.lineno, "startColumn": node.col_offset + 1,
            "endLine": getattr(node, "end_lineno", node.lineno), "endColumn": getattr(node, "end_col_offset", node.col_offset + 1) + 1}


def add(kind, key, label, parent=None, package=None, location=None, **attrs):
    id_ = uid(kind, key)
    value = dict(id=id_, kind=kind, label=label, attributes=dict(exported=False, **attrs))
    if parent:
        value["parentId"] = parent
    if package:
        value["packageId"] = package
    if location:
        value["source"] = location
    nodes.setdefault(id_, value)
    return id_


def link(kind, a, b, evidence=None, resolution=None):
    id_ = uid("edge", kind + ":" + a + ":" + b)
    edge = dict(id=id_, kind=kind, sourceId=a, targetId=b)
    if evidence:
        edge["evidence"] = evidence
    if resolution:
        edge["resolution"] = resolution
    edges.setdefault(id_, edge)


def note(code, message, location=None):
    value = dict(severity="warning", code=code, message=message)
    if location:
        value["source"] = location
    result["Diagnostics"].append(value)


def directory(path):
    if path in ("", "."):
        return root_id
    parent = directory(posixpath.dirname(path))
    id_ = add("directory", path, posixpath.basename(path), parent, relativePath=path)
    link("contains", parent, id_)
    return id_


root_id = add("module", payload["modulePath"], payload["modulePath"])
for file in payload["files"]:
    path = file["path"]
    raw = base64.b64decode(file["content"])
    result["Files"].append(dict(id=uid("file", path), relativePath=path, contentHash=file["hash"]))
    module = path[:-3].replace("/", ".")
    if module.startswith("src."):
        module = module[4:]
    if module.endswith(".__init__"):
        module = module[:-9]
    if module == "__init__":
        module = payload["rootName"]
    package = add("package", "python:" + path, module, directory(posixpath.dirname(path)),
                  language="python", relativePath=path, importPath=module)
    link("contains", nodes[package]["parentId"], package)
    modules.setdefault(module, []).append(package)
    try:
        encoding, _ = tokenize.detect_encoding(io.BytesIO(raw).readline)
        text = raw.decode(encoding)
        tree = ast.parse(raw, filename=path)
        nodes[package]["source"] = dict(fileId=uid("file", path), startLine=1, startColumn=1,
                                          endLine=max(1, len(text.splitlines())), endColumn=1)
        parsed.append(dict(path=path, module=module, package=package, tree=tree, text=text, is_init=path.endswith("__init__.py")))
    except (SyntaxError, UnicodeError, ValueError, LookupError) as error:
        note("python_parse", path + ": " + str(error))


def declarations(file, body, prefix="", owner=None):
    for node in body:
        if not isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            if isinstance(node, (ast.If, ast.Try)) and any(isinstance(x, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)) for x in ast.walk(node)):
                note("python_conditional_declaration", file["path"] + ": conditional declarations are omitted", source(file["path"], node))
            continue
        qual = prefix + node.name
        full = file["module"] + "." + qual
        kind = "class" if isinstance(node, ast.ClassDef) else "method" if owner else "closure" if prefix else "function"
        key = "python:" + file["path"] + ":" + qual
        if uid(kind, key) in nodes:
            key += ":" + str(node.lineno)
            note("python_redefinition", file["path"] + ": multiple declarations of " + qual, source(file["path"], node))
        if kind == "class":
            signature = "class " + node.name + ("(" + ", ".join(ast.unparse(x) for x in node.bases) + ")" if node.bases else "") + ":"
            members = [ast.unparse(x) for x in node.body if isinstance(x, ast.AnnAssign)]
        else:
            signature = ("async " if isinstance(node, ast.AsyncFunctionDef) else "") + "def " + node.name + "(" + ast.unparse(node.args) + ")" + (" -> " + ast.unparse(node.returns) if node.returns else "") + ":"
            members = []
        id_ = add(kind, key, node.name, owner or file["package"], file["package"], source(file["path"], node),
                  language="python", relativePath=file["path"], importPath=file["module"], signature=signature,
                  documentation=ast.get_docstring(node) or "", receiver=prefix.rstrip(".") if owner else "", members=members)
        nodes[id_]["attributes"]["exported"] = not node.name.startswith("_")
        symbols.setdefault(full, []).append(id_)
        if node.decorator_list:
            uncertain_symbols.add(id_)
            note("python_decorated_binding", file["path"] + ": decorated binding is not resolved as a direct call: " + qual, source(file["path"], node))
        link("declares_method" if kind == "method" else "contains", owner or file["package"], id_)
        if kind == "class":
            declarations(file, node.body, qual + ".", id_)
        else:
            functions.append((file, node, qual, id_))
            declarations(file, node.body, qual + ".")


for file in parsed:
    declarations(file, file["tree"].body)


def import_info(file, node):
    if isinstance(node, ast.Import):
        return [(alias.asname or alias.name.split(".")[0], alias.name if alias.asname else alias.name.split(".")[0], alias.name) for alias in node.names]
    base = file["module"].split(".") if file["is_init"] else file["module"].split(".")[:-1]
    if node.level:
        if node.level > len(base):
            note("python_relative_import", file["path"] + ": relative import escapes the discovered source root", source(file["path"], node))
            return []
        prefix = base[:len(base) - node.level + 1]
        module = ".".join(prefix + ([node.module] if node.module else []))
    else:
        module = node.module or ""
    return [(alias.asname or alias.name, module + "." + alias.name, module + "." + alias.name if module + "." + alias.name in modules else module) for alias in node.names]


for file in parsed:
    file["aliases"] = {}
    file["rebound"] = set()
    for statement in file["tree"].body:
        if isinstance(statement, (ast.Assign, ast.AnnAssign, ast.AugAssign)):
            file["rebound"].update(n.id for n in ast.walk(statement) if isinstance(n, ast.Name) and isinstance(n.ctx, ast.Store))
    for node in ast.walk(file["tree"]):
        if not isinstance(node, (ast.Import, ast.ImportFrom)):
            continue
        for name, target, dependency in import_info(file, node):
            if name == "*":
                note("python_star_import", file["path"] + ": wildcard import cannot resolve individual symbols", source(file["path"], node))
                continue
            ids = modules.get(dependency, [])
            if len(ids) == 1:
                target_id = ids[0]
            else:
                target_id = add("externalPackage", "python:" + dependency, dependency, language="python", importPath=dependency, external=True)
                if len(ids) > 1:
                    note("python_ambiguous_import", file["path"] + ": ambiguous local module " + dependency)
            link("imports", file["package"], target_id, source(file["path"], node))
    for node in file["tree"].body:
        if isinstance(node, (ast.Import, ast.ImportFrom)):
            file["aliases"].update((name, target) for name, target, _ in import_info(file, node) if name != "*")


def body_nodes(body):
    for node in body:
        yield node
        if not isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef, ast.Lambda)):
            yield from body_nodes(ast.iter_child_nodes(node))


for file, function, qual, caller in functions:
    aliases = dict(file["aliases"])
    walked = list(body_nodes(function.body))
    local_bound = set(file["rebound"])
    local_bound.update(arg.arg for arg in function.args.posonlyargs + function.args.args + function.args.kwonlyargs)
    if function.args.vararg:
        local_bound.add(function.args.vararg.arg)
    if function.args.kwarg:
        local_bound.add(function.args.kwarg.arg)
    local_bound.update(n.id for n in walked if isinstance(n, ast.Name) and isinstance(n.ctx, (ast.Store, ast.Del)))
    for n in walked:
        if isinstance(n, (ast.Import, ast.ImportFrom)):
            aliases.update((name, target) for name, target, _ in import_info(file, n) if name != "*")
    for call in (n for n in walked if isinstance(n, ast.Call)):
        printed = ast.unparse(call.func)
        root = printed.split(".")[0]
        targets = []
        if root not in local_bound:
            if root in aliases:
                targets = symbols.get(aliases[root] + printed[len(root):], [])
            else:
                scope = qual.split(".")
                for length in range(len(scope), -1, -1):
                    targets = symbols.get(".".join([file["module"]] + scope[:length] + [printed]), [])
                    if targets:
                        break
        if len(targets) == 1 and targets[0] not in uncertain_symbols:
            target = targets[0]
            if nodes[target]["kind"] == "class":
                constructor = symbols.get(file["module"] + "." + printed + ".__init__", [])
                if len(constructor) == 1:
                    target = constructor[0]
            link("calls", caller, target, source(file["path"], call), "static")
        elif root not in local_bound and ((root in aliases and not any(alias == aliases[root] or aliases[root].startswith(alias + ".") for alias in modules)) or root in vars(builtins)):
            target = add("externalCall", "python:" + printed, printed, language="python", external=True, resolution="external")
            link("calls", caller, target, source(file["path"], call), "external")
        else:
            target = add("dynamicCall", "python:" + caller + ":" + str(call.lineno) + ":" + str(call.col_offset), printed,
                         file["package"], file["package"], source(file["path"], call), language="python", resolution="dynamic",
                         documentation="Python runtime binding, receiver types, and monkey-patching are not resolved statically.")
            link("dynamic_call", caller, target, source(file["path"], call), "dynamic")

result["Nodes"] = list(nodes.values())
result["Edges"] = list(edges.values())
json.dump(result, sys.stdout, ensure_ascii=True)
