// App.jsx
import React, { useEffect, useRef, useState, useCallback } from "react";

/* ===========================
   MODAL: LOGIN
   =========================== */
const LoginModal = ({ open, onClose, apiUrl, apiStatus, onLoggedIn }) => {
  const [id, setId] = useState(localStorage.getItem("gd_last_id") || "");
  const [user, setUser] = useState(localStorage.getItem("gd_last_user") || "root");
  const [pass, setPass] = useState("");
  const [remember, setRemember] = useState(localStorage.getItem("gd_remember") === "1");
  const [loading, setLoading] = useState(false);
  const [msg, setMsg] = useState("");

  useEffect(() => { if (!open) { setMsg(""); setPass(""); } }, [open]);

  const parseLoginOK = (raw) => /Logueado\s+con\s+Exito/i.test(raw);

  const handleSubmit = async (e) => {
    e?.preventDefault?.();
    if (apiStatus !== "online") { setMsg("API desconectada."); return; }
    if (!id.trim() || !user.trim() || !pass) { setMsg("Completa todos los campos."); return; }

    setLoading(true);
    setMsg("Conectando...");
    try {
      const command = `login -user=${user} -pass=${pass} -id=${id}`;
      const res = await fetch(`${apiUrl}/execute`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ command }),
      });
      const data = await res.json();
      const output = (data?.output || "").toString();
      const ok = parseLoginOK(output);
      setMsg(output);

      if (ok) {
        if (remember) {
          localStorage.setItem("gd_last_id", id);
          localStorage.setItem("gd_last_user", user);
          localStorage.setItem("gd_remember", "1");
        } else {
          localStorage.removeItem("gd_last_id");
          localStorage.removeItem("gd_last_user");
          localStorage.removeItem("gd_remember");
        }
        onLoggedIn({ user, id });
        onClose();
      }
    } catch (err) {
      setMsg(`Error: ${err?.message || err}`);
    } finally {
      setLoading(false);
    }
  };

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/70" onClick={onClose} />
      <div className="relative w-full max-w-md bg-black border border-green-600 rounded-xl p-6 z-10">
        <h2 className="text-2xl font-bold text-green-400 mb-4 text-center">Login</h2>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="blocks text-green-400 mb-1">ID Partición</label>
            <input
              className="w-full p-2 bg-black border border-green-500 rounded text-green-300 outline-none focus:ring-2 focus:ring-green-400"
              placeholder="341A"
              value={id}
              onChange={(e) => setId(e.target.value)}
            />
          </div>
          <div>
            <label className="blocks text-green-400 mb-1">Usuario</label>
            <input
              className="w-full p-2 bg-black border border-green-500 rounded text-green-300 outline-none focus:ring-2 focus:ring-green-400"
              placeholder="root"
              value={user}
              onChange={(e) => setUser(e.target.value)}
            />
          </div>
          <div>
            <label className="block text-green-400 mb-1">Contraseña</label>
            <input
              type="password"
              className="w-full p-2 bg-black border border-green-500 rounded text-green-300 outline-none focus:ring-2 focus:ring-green-400"
              placeholder="•••••••"
              value={pass}
              onChange={(e) => setPass(e.target.value)}
            />
          </div>

          <label className="flex items-center gap-2 text-green-400 text-sm">
            <input type="checkbox" checked={remember} onChange={(e) => setRemember(e.target.checked)} />
            Recordar usuario
          </label>

          <div className="flex gap-2 pt-2">
            <button
              type="submit"
              disabled={loading || apiStatus !== "online"}
              className={`flex-1 px-4 py-2 rounded transition ${loading || apiStatus !== "online"
                  ? "bg-gray-800 text-gray-500 cursor-not-allowed"
                  : "bg-green-500 text-black font-semibold hover:bg-green-400"
                }`}
            >
              {loading ? "Ingresando..." : "Ingresar"}
            </button>
            <button type="button" onClick={onClose} className="px-4 py-2 rounded border border-green-500 text-green-400 hover:bg-gray-800">
              Cancelar
            </button>
          </div>
        </form>

        {!!msg && (
          <div className="mt-4 max-h-40 overflow-auto bg-black border border-green-700 rounded p-2 text-xs text-green-300">
            <pre>{msg}</pre>
          </div>
        )}
      </div>
    </div>
  );
};

/* ===========================
   API CONNECTOR
   =========================== */
const ApiConnector = ({ apiUrl, setApiUrl, apiStatus, checkApiStatus }) => {
  const statusMap = {
    online: { text: "Conectado", color: "bg-green-500", textColor: "text-green-400" },
    offline: { text: "Desconectado", color: "bg-red-500", textColor: "text-red-400" },
    connecting: { text: "Conectando...", color: "bg-yellow-500", textColor: "text-yellow-400" },
  };
  const currentStatus = statusMap[apiStatus] || statusMap.offline;

  return (
    <div className="flex items-center gap-4 p-3 bg-black border border-green-700 rounded-lg">
      <label htmlFor="api-url" className="text-green-400 font-semibold">API Backend:</label>
      <input
        id="api-url"
        type="text"
        value={apiUrl}
        onChange={(e) => setApiUrl(e.target.value)}
        placeholder="http://localhost:3001"
        className="flex-grow p-2 bg-black border border-green-500 rounded text-green-300 outline-none focus:ring-2 focus:ring-green-400"
      />
      <button
        onClick={checkApiStatus}
        disabled={apiStatus === 'connecting'}
        className="px-4 py-2 bg-green-600 text-black font-semibold rounded hover:bg-green-500 transition disabled:bg-gray-600 disabled:cursor-not-allowed"
      >
        {apiStatus === 'connecting' ? 'Probando...' : 'Probar'}
      </button>
      <div className="flex items-center gap-2">
        <span className={`w-3 h-3 rounded-full ${currentStatus.color}`}></span>
        <span className={`font-semibold ${currentStatus.textColor}`}>{currentStatus.text}</span>
      </div>
    </div>
  );
};

/* ===========================
   HEADER
   =========================== */
const Header = ({ activeTab, setActiveTab, apiUrl, setApiUrl, apiStatus, checkApiStatus, session, onOpenLogin, onLogout }) => {
  const tabs = [
    { id: 'console', label: 'Consola', icon: '💻' },
    { id: 'reports', label: 'Reportes', icon: '📊' },
    { id: 'viewer', label: 'Visualizador', icon: '🗂️' },
    { id: 'about', label: 'Acerca de', icon: 'ℹ️' }
  ];

  return (
    <header className="bg-black border-b border-green-500">
      <div className="max-w-6xl mx-auto px-6 py-4 space-y-4">
        <div className="flex justify-between items-start">
          <div>
            <h1 className="text-4xl font-bold text-green-400">GoDisk</h1>
            <h2 className="text-2xl font-bold text-green-600">Proyecto MIA 201902672</h2>
          </div>
          <div className="flex items-center gap-3">
            {session?.loggedIn ? (
              <>
                <span className="px-3 py-1 rounded-full border border-green-500 text-green-300 text-sm">
                  {session.user}@{session.id}
                </span>
                <button onClick={onLogout} className="px-3 py-1 bg-red-500 text-black rounded hover:bg-red-400">
                  Cerrar sesión
                </button>
              </>
            ) : (
              <button
                onClick={onOpenLogin}
                disabled={apiStatus !== 'online'}
                className={`px-4 py-2 rounded ${apiStatus !== 'online' ? 'bg-gray-800 text-gray-500 cursor-not-allowed' : 'bg-green-500 text-black font-semibold hover:bg-green-400'}`}
              >
                Iniciar sesión
              </button>
            )}
          </div>
        </div>

        <ApiConnector apiUrl={apiUrl} setApiUrl={setApiUrl} apiStatus={apiStatus} checkApiStatus={checkApiStatus} />

        <nav className="flex space-x-1">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex items-center space-x-2 px-4 py-2 rounded-lg transition ${activeTab === tab.id ? 'bg-green-500 text-black font-semibold' : 'text-green-400 hover:bg-gray-800 hover:text-green-300'}`}
            >
              <span className="text-lg">{tab.icon}</span>
              <span>{tab.label}</span>
            </button>
          ))}
        </nav>
      </div>
    </header>
  );
};

/* ===========================
   FILE MENU
   =========================== */
const FileMenu = ({ onOpenFile, onSaveFile, onSaveAsFile, currentFileName }) => {
  const [isOpen, setIsOpen] = useState(false);
  return (
    <div className="relative">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center space-x-2 px-4 py-2 bg-gray-900 text-green-400 border border-green-500 rounded hover:bg-green-500 hover:text-black transition"
      >
        <span>📄</span><span>Archivo</span>
      </button>
      {isOpen && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setIsOpen(false)} />
          <div className="absolute top-full left-0 mt-1 bg-black border border-green-500 rounded-lg shadow-lg z-20 min-w-48">
            <button onClick={() => { onOpenFile(); setIsOpen(false); }} className="w-full flex items-center space-x-2 px-4 py-2 text-left text-green-400 hover:bg-gray-800 rounded-t-lg">
              <span>📂</span><span>Abrir archivo...</span>
            </button>
            <button
              onClick={() => { onSaveFile(); setIsOpen(false); }}
              disabled={!currentFileName}
              className="w-full flex items-center space-x-2 px-4 py-2 text-left text-green-400 hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <span>💾</span><span>Guardar</span>
              {currentFileName && <span className="text-xs text-green-600">({currentFileName})</span>}
            </button>
            <button onClick={() => { onSaveAsFile(); setIsOpen(false); }} className="w-full flex items-center space-x-2 px-4 py-2 text-left text-green-400 hover:bg-gray-800 rounded-b-lg">
              <span>⬇️</span><span>Guardar como...</span>
            </button>
          </div>
        </>
      )}
    </div>
  );
};

/* ===========================
   CONSOLA
   =========================== */
const ConsoleTab = ({ apiUrl, apiStatus, session }) => {
  const [entradaComandos, setEntradaComandos] = useState("");
  const [salidaComandos, setSalidaComandos] = useState("");
  const [loading, setLoading] = useState(false);
  const [currentFileName, setCurrentFileName] = useState("");
  const outRef = useRef(null);
  const fileInputRef = useRef(null);

  useEffect(() => { outRef.current?.scrollTo({ top: outRef.current.scrollHeight }); }, [salidaComandos, loading]);

  const requiereSesion = (cmd) => /^(mkfile|mkdir|mkgrp|rmgrp|mkusr|rmusr|chgrp|cat)\b/i.test(cmd.trim());

  const manejarEjecucion = async () => {
    const command = entradaComandos.trim();
    if (!command || loading || apiStatus !== 'online') return;

    if (requiereSesion(command) && !session?.loggedIn) {
      setSalidaComandos((prev) =>
        prev +
        `\n⚠ Requiere sesión: inicia sesión para ejecutar «${command.split(/\s+/)[0]}».\n` +
        `Usa el botón “Iniciar sesión” en la parte superior.\n`
      );
      return;
    }

    try {
      setLoading(true);
      setSalidaComandos((prev) => prev + `\n▶ Ejecutando...\n`);
      const respuesta = await fetch(`${apiUrl}/execute`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ command }),
      });
      if (!respuesta.ok) {
        const t = await respuesta.text();
        throw new Error(t || `HTTP ${respuesta.status}`);
      }
      const datos = await respuesta.json();
      const out = (datos?.output ?? "").toString().replace(/\r/g, "");
      setSalidaComandos((prev) => prev + out + `\n✔ Finalizado\n`);
    } catch (e) {
      setSalidaComandos((prev) => prev + `\n✖ Error: ${e.message || e}\n`);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    const h = (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key === "Enter") { e.preventDefault(); manejarEjecucion(); }
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [entradaComandos, loading, apiStatus, session?.loggedIn]);

  useEffect(() => {
    const onLogout = () => setSalidaComandos(prev => prev + "\n🔒 Sesión cerrada correctamente.\n");
    window.addEventListener("gd-logout", onLogout);
    return () => window.removeEventListener("gd-logout", onLogout);
  }, []);

  const handleOpenFile = () => fileInputRef.current?.click();
  const manejarArchivo = async (evento) => {
    const archivo = evento.target.files?.[0];
    if (!archivo) return;
    const contenido = await archivo.text();
    setEntradaComandos(contenido);
    setCurrentFileName(archivo.name);
    evento.target.value = "";
  };
  const handleSaveFile = () => {
    if (!currentFileName) { handleSaveAsFile(); return; }
    const blob = new Blob([entradaComandos], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a'); a.href = url; a.download = currentFileName; a.click();
    URL.revokeObjectURL(url);
  };
  const handleSaveAsFile = () => {
    const fileName = prompt('Nombre del archivo:', currentFileName || 'script.smia');
    if (!fileName) return;
    const blob = new Blob([entradaComandos], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a'); a.href = url; a.download = fileName; a.click();
    URL.revokeObjectURL(url);
    setCurrentFileName(fileName);
  };
  const limpiarSalida = () => setSalidaComandos("");
  const copiarSalida = async () => {
    try { await navigator.clipboard.writeText(salidaComandos); alert('Salida copiada al portapapeles'); } catch { alert('Error al copiar al portapapeles'); }
  };
  const onDrop = async (e) => { e.preventDefault(); const file = e.dataTransfer.files?.[0]; if (!file) return; setEntradaComandos(await file.text()); setCurrentFileName(file.name); };
  const onDragOver = (e) => e.preventDefault();

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center gap-3 p-4 bg-gray-900 border border-green-500 rounded-lg">
        <FileMenu onOpenFile={handleOpenFile} onSaveFile={handleSaveFile} onSaveAsFile={handleSaveAsFile} currentFileName={currentFileName} />
        <div className="w-px h-8 bg-green-500" />
        <button
          onClick={manejarEjecucion}
          disabled={loading || !entradaComandos.trim() || apiStatus !== 'online'}
          className={`flex items-center space-x-2 px-4 py-2 rounded transition ${loading || apiStatus !== 'online' ? "bg-gray-800 text-gray-500 cursor-not-allowed" : "bg-green-500 text-black font-semibold hover:bg-green-400"}`}
        >
          <span>{loading ? "⏳" : "▶️"}</span>
          <span>{loading ? "Ejecutando..." : "Ejecutar"}</span>
        </button>
        {apiStatus !== 'online' && <span className="text-xs text-red-400">API Desconectada</span>}

        <div className="ml-auto">
          {session?.loggedIn ? (
            <span className="px-3 py-1 rounded-full border border-green-500 text-green-300 text-xs">
              Sesión: {session.user}@{session.id}
            </span>
          ) : (
            <span className="text-xs text-green-600">No hay sesión activa</span>
          )}
        </div>
      </div>

      <div onDrop={onDrop} onDragOver={onDragOver} className="rounded-lg border border-green-500 bg-gray-900 overflow-hidden">
        <div className="bg-black px-4 py-2 border-b border-green-500">
          <span className="text-sm text-green-400">{currentFileName || "Sin título"} - Editor de comandos</span>
        </div>
        <textarea
          className="w-full h-64 p-4 bg-transparent text-green-400 placeholder-green-700 outline-none resize-none"
          value={entradaComandos}
          onChange={(e) => setEntradaComandos(e.target.value)}
          placeholder={`Escribe comandos aquí o arrastra un archivo .smia...`}
        />
      </div>

      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold text-green-400">Salida de la consola</h3>
        <div className="flex gap-2">
          <button onClick={copiarSalida} disabled={!salidaComandos} className="px-3 py-1 text-sm border border-green-500 text-green-400 rounded hover:bg-green-500 hover:text-black transition disabled:opacity-50">📋 Copiar</button>
          <button onClick={limpiarSalida} disabled={!salidaComandos} className="px-3 py-1 text-sm border border-red-500 text-red-400 rounded hover:bg-red-500 hover:text-black transition disabled:opacity-50">🗑️ Limpiar</button>
        </div>
      </div>
      <div ref={outRef} className="bg-black border border-green-500 rounded-lg p-4 h-80 overflow-auto text-green-400 font-mono">
        <pre>{salidaComandos || "# Salida de comandos aparecerá aquí..."}</pre>
      </div>

      <input ref={fileInputRef} type="file" accept=".smia,.txt" onChange={manejarArchivo} className="hidden" />
    </div>
  );
};

/* ===========================
   REPORTES (Endpoint + Fallback)
   =========================== */
const ReportsTab = ({ apiUrl, apiStatus, session }) => {
  const REPORTS = [
    { key: "BLOCK", label: "Reporte BLOCK", backend: "blocks", needsRuta: true, special: null },
    { key: "BM_BLOCK", label: "Reporte BM_BLOCK", backend: null, needsRuta: false, special: "bm_blocks" },
    { key: "BM_INODES", label: "Reporte BM_INODES", backend: "bm_inodes", needsRuta: false, special: null },
    { key: "DISK", label: "Reporte DISK", backend: "disk", needsRuta: true, special: null },
    { key: "FILE", label: "Reporte FILE", backend: "file", needsRuta: true, special: null },
    { key: "INODES", label: "Reporte INODES", backend: "inodes", needsRuta: false, special: null },
    { key: "LS", label: "Reporte LS", backend: "ls", needsRuta: true, special: null },
    { key: "MBR", label: "Reporte MBR", backend: "mbr", needsRuta: false, special: null },
    { key: "SB", label: "Reporte SB", backend: "sb", needsRuta: false, special: null },
    { key: "TREE", label: "Reporte TREE", backend: "tree", needsRuta: false, special: null },
  ];

  const [loading, setLoading] = useState(false);
  const [log, setLog] = useState("");
  const [items, setItems] = useState([]); // [{type,label,url,ext,isLocal,localPath}]
  const [form, setForm] = useState({
    reportKey: "BLOCK",
    id: session?.id || "",
    ruta: "/users.txt",
  });

  useEffect(() => { if (session?.id) setForm(f => ({ ...f, id: session.id })); }, [session?.id]);
  const setF = (k, v) => setForm(prev => ({ ...prev, [k]: v }));

  const selected = REPORTS.find(r => r.key === form.reportKey) || REPORTS[0];
  const needsRuta = !!selected.needsRuta;

  const guessExt = (urlOrPath, fallback) => {
    const m = (urlOrPath || "").toLowerCase().match(/\.(jpg|jpeg|png|gif|svg|pdf|txt)(\?|#|$)/);
    return m ? m[1] : (fallback || "");
  };

  // 1) Endpoint genérico /reports/generate
  const callGenerateEndpoint = async () => {
    if (!selected.backend) {
      // no aplica para especiales como BM_BLOCK
      throw Object.assign(new Error("No backend name"), { status: 501 });
    }
    const body = {
      id: form.id.trim(),
      name: selected.backend,                    // blocks, ls, sb, inodes, tree, mbr, file, disk, bm_inodes
      ruta: needsRuta ? form.ruta : "",
    };
    const res = await fetch(`${apiUrl}/reports/generate`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body), // <— plano, sin envolver en { body: ... }
    });
    if (!res.ok) {
      const txt = await res.text();
      const err = new Error(txt || `HTTP ${res.status}`);
      err.status = res.status;
      throw err;
    }
    return res.json(); // { ok, publicUrl?, localPath?, contentType?, message? }
  };

  // 2) Endpoint especial (por ahora sólo BM_BLOCK)
  const callSpecialEndpoint = async () => {
    // BM_BLOCK → /reports/generate-bitmap-blocks {id}
    if (selected.special === "bm_blocks") {
      const res = await fetch(`${apiUrl}/reports/generate-bitmap-blocks`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id: form.id.trim() }),
      });
      if (!res.ok) {
        const txt = await res.text();
        const err = new Error(txt || `HTTP ${res.status}`);
        err.status = res.status;
        throw err;
      }
      return res.json(); // { ok, publicUrl?, localPath?, contentType: text/plain, message? }
    }
    // Si más adelante agregas otros especiales, manéjalos aquí.
    const err = new Error("Endpoint especial no implementado");
    err.status = 501;
    throw err;
  };

  // 3) Fallback a /execute con -path local (sirve para cualquier name)
  const fallbackViaExecute = async () => {
    const stamp = new Date().toISOString().replace(/[:.]/g, "-");

    // Heurística de extensión:
    //  - bitmap -> txt
    //  - file/ls/blocks/sb/inodes/tree/mbr/disk -> jpg
    let ext = "jpg";
    if (selected.key === "BM_BLOCK" || selected.key === "BM_INODES") ext = "txt";

    const nameForCmd = (selected.backend || selected.key).toLowerCase();
    const outPath = `/tmp/godisk_reports/${nameForCmd}_${stamp}.${ext}`;

    let cmd = `rep -id=${form.id.trim()} -name=${nameForCmd} -path="${outPath}"`;
    if (needsRuta) cmd += ` -ruta="${form.ruta}"`;

    const r = await fetch(`${apiUrl}/execute`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ command: cmd }),
    });
    if (!r.ok) throw new Error((await r.text()) || `HTTP ${r.status}`);
    const data = await r.json();
    const out = String(data?.output || "");

    // Extraer URL Pública si el backend también subió a S3
    const s3Match = out.match(/URL Pública:\s*(https?:\/\/\S+)/i);
    const publicUrl = s3Match ? s3Match[1] : null;

    return {
      ok: true,
      publicUrl,
      localPath: publicUrl ? null : outPath,
      message: out,
    };
  };

  const handleGenerate = async () => {
    if (apiStatus !== "online") { setLog("API desconectada."); return; }
    if (!form.id.trim()) { setLog(p => p + "✖ Debe indicar ID de partición.\n"); return; }
    if (needsRuta && !form.ruta.trim()) { setLog(p => p + "✖ Debe indicar la ruta lógica.\n"); return; }

    setLoading(true);
    setLog(p => p + `\n▶ Generando reporte (${selected.key})…\n`);
    try {
      let payload;

      // 1) Intentar endpoint especial
      if (selected.special) {
        try {
          payload = await callSpecialEndpoint();
        } catch (e) {
          if (e?.status !== 404 && e?.status !== 501) throw e;
          setLog(p => p + "ℹ Endpoint especial no disponible. Probando el genérico…\n");
        }
      }

      // 2) Si no hubo especial o falló, intentar el genérico
      if (!payload) {
        try {
          payload = await callGenerateEndpoint();
        } catch (e) {
          if (e?.status === 404 || e?.status === 501) {
            setLog(p => p + "ℹ Endpoint /reports/generate no disponible. Usando fallback /execute…\n");
            payload = await fallbackViaExecute();
          } else {
            throw e;
          }
        }
      }

      const msg = payload?.message ? `\n${payload.message}\n` : "";
      setLog(p => p + msg + "✔ Listo.\n");

      const url = payload?.publicUrl || null;
      const localPath = payload?.localPath || null;
      const label = `${selected.key} ${new Date().toLocaleString()}`;

      if (!url && !localPath) {
        setLog(p => p + "⚠ No se recibió URL pública ni ruta local.\n");
        return;
      }

      // Deducir extensión para vista previa
      const ext = payload?.contentType?.includes("text/plain")
        ? "txt"
        : guessExt(url || localPath, selected.key.startsWith("BM_") ? "txt" : "jpg");

      const item = url
        ? { type: selected.key, label, url, ext, isLocal: false }
        : { type: selected.key, label, url: `${apiUrl}/reports/proxy?path=${encodeURIComponent(localPath)}`, ext, isLocal: true, localPath };

      setItems(prev => [item, ...prev]);
    } catch (e) {
      setLog(p => p + `✖ Error: ${e.message || e}\n`);
    } finally {
      setLoading(false);
    }
  };

  const Preview = ({ item }) => {
    if (!item?.url) return null;
    const ext = item.ext;
    if (["jpg", "jpeg", "png", "gif", "svg"].includes(ext)) {
      return (
        <div className="aspect-[4/3] overflow-hidden rounded border border-green-800 bg-gray-900 mb-2">
          <img src={item.url} alt={item.label} className="w-full h-full object-contain" />
        </div>
      );
    }
    if (ext === "pdf") {
      return (
        <div className="h-40 overflow-hidden rounded border border-green-800 bg-gray-900 mb-2">
          <iframe title={item.label} src={item.url} className="w-full h-full" />
        </div>
      );
    }
    if (ext === "txt") {
      return (
        <div className="p-2 h-40 overflow-auto rounded border border-green-800 bg-gray-900 mb-2 text-xs text-green-300">
          <em>Archivo de texto. Si no carga por CORS, usa “Abrir”.</em>
        </div>
      );
    }
    return (
      <div className="aspect-[4/3] flex items-center justify-center rounded border border-green-800 bg-gray-900 mb-2 text-green-500">
        Sin vista previa ({ext || "desconocido"})
      </div>
    );
  };

  return (
    <div className="space-y-6">
      <div className="rounded-xl border border-green-600 p-6 bg-gray-950 space-y-4">
        <h2 className="text-2xl font-bold text-green-400">Generar reporte</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div>
            <label className="block text-green-400 mb-1">Tipo de reporte</label>
            <select
              value={form.reportKey}
              onChange={(e) => setF("reportKey", e.target.value)}
              className="w-full p-2 bg-black border border-green-500 rounded text-green-300"
            >
              {REPORTS.map(r => (
                <option key={r.key} value={r.key}>{r.label}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-green-400 mb-1">ID Partición</label>
            <input
              value={form.id}
              onChange={(e) => setF("id", e.target.value)}
              placeholder="721A"
              className="w-full p-2 bg-black border border-green-500 rounded text-green-300"
            />
          </div>

          {needsRuta && (
            <div>
              <label className="block text-green-400 mb-1">
                {selected.key === "DISK" ? "Ruta del disco (.mia)" : "Ruta lógica"}
              </label>
              <input
                value={form.ruta}
                onChange={(e) => setF("ruta", e.target.value)}
                placeholder={selected.key === "DISK" ? "/home/ubuntu/Calificacion_MIA/" : "/home/ubuntu/Calificacion_MIA/Discos/Disco1.mia"}
                className="w-full p-2 bg-black border border-green-500 rounded text-green-300"
              />
            </div>
          )}
        </div>

        <div className="pt-2">
          <button
            onClick={handleGenerate}
            disabled={loading || apiStatus !== "online"}
            className={`px-4 py-2 rounded ${loading || apiStatus !== 'online' ? 'bg-gray-800 text-gray-500 cursor-not-allowed' : 'bg-green-500 text-black font-semibold hover:bg-green-400'}`}
          >
            {loading ? "Generando..." : "Generar"}
          </button>
        </div>
      </div>

      <div className="rounded-xl border border-green-600 p-6 bg-gray-950">
        <h3 className="text-xl font-bold text-green-400 mb-4">Reportes recientes (sesión actual)</h3>
        {items.length === 0 ? (
          <div className="text-green-600 text-sm">Aún no has generado reportes.</div>
        ) : (
          <div className="flex flex-wrap gap-4">
            {items.map((it, idx) => (
              <div key={idx} className="w-60 p-3 border border-green-700 rounded-lg bg-black">
                <div className="text-green-300 text-sm mb-2 truncate">{it.label}</div>
                <Preview item={it} />
                <div className="text-[11px] text-green-600 space-y-1">
                  <div>Tipo: <span className="text-green-300">{it.type}</span></div>
                  <div className="truncate">{it.isLocal ? "Local" : "S3"}: <a href={it.url} className="underline" target="_blank" rel="noreferrer">Abrir</a></div>
                </div>
                <div className="mt-2 flex gap-2">
                  <a
                    href={it.url}
                    download
                    className="px-2 py-1 text-xs border border-green-500 rounded text-green-400 hover:bg-green-500 hover:text-black"
                  >
                    Descargar
                  </a>
                  <button
                    onClick={async () => { try { await navigator.clipboard.writeText(it.url); alert("URL copiada ✔"); } catch { alert("No se pudo copiar"); } }}
                    className="px-2 py-1 text-xs border border-green-500 rounded text-green-400 hover:bg-green-500 hover:text-black"
                  >
                    Copiar URL
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="rounded-xl border border-green-600 bg-black p-4">
        <h4 className="font-semibold text-green-400 mb-2">Bitácora</h4>
        <pre className="text-green-300 text-xs whitespace-pre-wrap">{log || "Aquí verás el resultado/URLs de los reportes…"}</pre>
      </div>
    </div>
  );
};


/* ===========================
   VISUALIZADOR: UI
   =========================== */
const InfoPill = ({ children }) => (
  <span className="px-2 py-0.5 text-xs rounded-full border border-green-600 text-green-300">{children}</span>
);

const EntryCard = ({ entry, onOpen }) => {
  const isDir = entry.type === "dir";
  return (
    <button
      onClick={() => onOpen(entry)}
      title={isDir ? "Abrir carpeta" : "Abrir archivo"}
      className="group w-44 p-3 rounded-lg border border-green-700 bg-black hover:bg-gray-900 transition text-left"
    >
      <div className="flex items-center justify-center w-14 h-14 rounded-lg bg-gray-800 mb-2">
        <span className="text-3xl">{isDir ? "📁" : "📄"}</span>
      </div>
      <div className="font-semibold text-green-200 truncate">{entry.name}</div>
      <div className="text-[11px] text-green-500 mt-1 space-y-0.5">
        <div>Permisos: <span className="text-green-300">{entry.perm}</span></div>
        <div>UID:{entry.uid} • GID:{entry.gid}</div>
        {!isDir && <div>Tamaño: <span className="text-green-300">{formatBytes(entry.size)}</span></div>}
        <div>Inodo: {entry.inode}</div>
      </div>
    </button>
  );
};

const formatBytes = (n) => {
  if (!n) return "0 B";
  const u = ["B", "KiB", "MiB", "GiB", "TiB"];
  let i = 0, v = n;
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++; }
  return `${v.toFixed(1)} ${u[i]}`;
};

const Breadcrumbs = ({ path, onJump }) => {
  const parts = path === "/" ? [""] : path.split("/").filter(Boolean);
  const segs = ["/", ...parts.map((_, i) => "/" + parts.slice(0, i + 1).join("/"))];
  const labels = ["/", ...parts];
  return (
    <div className="flex items-center gap-1 text-green-200 text-sm">
      {segs.map((p, i) => (
        <span key={p} className="flex items-center gap-1">
          <button onClick={() => onJump(p)} className="hover:underline">{labels[i]}</button>
          {i < segs.length - 1 && <span className="text-green-600">/</span>}
        </span>
      ))}
    </div>
  );
};

const FileViewerModal = ({ open, onClose, meta, content }) => {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/70" onClick={onClose} />
      <div className="relative w-full max-w-3xl bg-black border border-green-600 rounded-xl p-6 z-10">
        <h3 className="text-xl font-bold text-green-400 mb-3">Visualizador de Archivos</h3>
        <div className="flex items-center gap-2 text-sm text-green-300 mb-2">
          <InfoPill>{meta?.name}</InfoPill>
          <InfoPill>Tamaño: {meta?.size} B</InfoPill>
          <InfoPill>Perm: {meta?.perm}</InfoPill>
          <InfoPill>UID:{meta?.uid} GID:{meta?.gid}</InfoPill>
        </div>
        <textarea readOnly className="w-full h-72 p-3 bg-black border border-green-700 rounded text-green-300 font-mono" value={content || ""} />
        <div className="mt-4 text-right">
          <button onClick={onClose} className="px-4 py-2 rounded border border-green-500 text-green-400 hover:bg-gray-800">Cerrar</button>
        </div>
      </div>
    </div>
  );
};

const DiskCard = ({ disk, onSelect }) => (
  <button onClick={() => onSelect(disk)} className="group w-48 p-4 rounded-xl border border-green-600 bg-black hover:bg-gray-900 transition text-left">
    <div className="flex items-center justify-center w-16 h-16 rounded-lg bg-gray-800 mb-3"><span className="text-3xl">💽</span></div>
    <div className="font-semibold text-green-300">{disk.name}</div>
    <div className="text-xs text-green-500 mt-1">Capacidad: <span className="text-green-300">{disk.sizePretty}</span></div>
    <div className="text-xs text-green-500">Fit: <span className="text-green-300">{disk.fit || "—"}</span></div>
    <div className="text-xs text-green-500">Particiones montadas: <span className="text-green-300">{disk.mounted?.length || 0}</span></div>
  </button>
);

const DiskSelector = ({ apiUrl, apiStatus, onSelected }) => {
  const [loading, setLoading] = useState(false);
  const [disks, setDisks] = useState([]);
  const [err, setErr] = useState("");

  const load = async () => {
    if (apiStatus !== 'online') { setErr("API desconectada"); return; }
    setLoading(true); setErr("");
    try {
      const r = await fetch(`${apiUrl}/disks`);
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      const json = await r.json();
      setDisks(Array.isArray(json) ? json : []);
    } catch (e) { setErr(e?.message || "Error al cargar discos"); }
    finally { setLoading(false); }
  };
  useEffect(() => { load(); }, [apiUrl, apiStatus]);

  return (
    <div className="space-y-6">
      <div className="rounded-xl border border-green-600 p-6 bg-gray-950">
        <h2 className="text-2xl font-bold text-green-400 text-center">Visualizador del Sistema de Archivos</h2>
        <p className="text-sm text-green-500 text-center mt-1">Seleccione el disco que desea visualizar</p>
        <div className="mt-6 flex flex-wrap gap-4">
          {loading && <div className="text-green-400 text-sm">Cargando discos…</div>}
          {err && <div className="text-red-400 text-sm">✖ {err}</div>}
          {!loading && !err && disks.length === 0 && <div className="text-green-500 text-sm">No se encontraron discos .mia</div>}
          {disks.map(d => <DiskCard key={d.path} disk={d} onSelect={onSelected} />)}
        </div>
      </div>
    </div>
  );
};

const PartitionCard = ({ part, onSelect }) => (
  <button onClick={() => onSelect(part)} className="group w-52 p-4 rounded-lg border border-green-600 bg-black hover:bg-gray-900 transition text-left">
    <div className="flex items-center justify-center w-14 h-14 rounded-lg bg-gray-800 mb-2"><span className="text-3xl">🧩</span></div>
    <div className="font-semibold text-green-200 truncate">{part.name || "(sin nombre)"}</div>
    <div className="text-[11px] text-green-500 mt-1 space-y-0.5">
      <div>Tamaño: <span className="text-green-300">{part.sizePretty}</span></div>
      <div>Fit: <span className="text-green-300">{part.fit || "—"}</span></div>
      <div>Estado: <span className="text-green-300">{part.status === "1" ? "Activa" : "Inactiva"}</span></div>
      <div>Tipo: {part.type}</div>
    </div>
  </button>
);

const PartitionSelector = ({ apiUrl, apiStatus, disk, onSelected, onBack }) => {
  const [parts, setParts] = useState([]);
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(false);

  const load = async () => {
    if (apiStatus !== 'online') { setErr("API desconectada"); return; }
    setLoading(true); setErr("");
    try {
      const url = new URL(`${apiUrl}/disk/partitions`);
      url.searchParams.set("diskPath", disk.path);
      const r = await fetch(url);
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      const json = await r.json();
      setParts(Array.isArray(json) ? json : []);
    } catch (e) { setErr(e?.message || "Error al cargar particiones"); }
    finally { setLoading(false); }
  };
  useEffect(() => { load(); }, [disk?.path, apiUrl, apiStatus]);

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <button onClick={onBack} className="px-3 py-1 border border-green-600 rounded text-green-400 hover:bg-gray-900">← Volver a selección de discos</button>
        <InfoPill>{disk.name}</InfoPill>
        <InfoPill>{disk.sizePretty}</InfoPill>
        <InfoPill>Fit {disk.fit || "—"}</InfoPill>
      </div>

      <div className="rounded-xl border border-green-600 p-6 bg-gray-950">
        <h3 className="text-lg font-bold text-green-400 text-center">Seleccione la partición que desea visualizar</h3>
        <div className="mt-6 flex flex-wrap gap-4">
          {loading && <div className="text-green-400 text-sm">Cargando particiones…</div>}
          {err && <div className="text-red-400 text-sm">✖ {err}</div>}
          {!loading && !err && parts.length === 0 && <div className="text-green-500 text-sm">No hay particiones detectadas</div>}
          {parts.map(p => <PartitionCard key={`${p.name}-${p.start}`} part={p} onSelect={onSelected} />)}
        </div>
      </div>
    </div>
  );
};

const Explorer = ({ apiUrl, disk, part }) => {
  const [path, setPath] = useState("/");
  const [entries, setEntries] = useState([]);
  const [loading, setLoading] = useState(false);
  const [err, setErr] = useState("");
  const [fileMeta, setFileMeta] = useState(null);
  const [fileContent, setFileContent] = useState("");
  const [openFile, setOpenFile] = useState(false);

  const fetchList = useCallback(async (targetPath = path) => {
    setLoading(true); setErr("");
    try {
      const url = new URL(`${apiUrl}/viewer/list`);
      url.searchParams.set("diskPath", disk.path);
      url.searchParams.set("partName", part.name);
      url.searchParams.set("path", targetPath || "/");
      const r = await fetch(url);
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      const data = await r.json();
      setEntries(Array.isArray(data) ? data : []);
    } catch (e) { setErr(e?.message || "Error al listar"); }
    finally { setLoading(false); }
  }, [apiUrl, disk?.path, part?.name, path]);

  useEffect(() => { setPath("/"); }, [disk?.path, part?.name]);
  useEffect(() => { fetchList("/"); }, [disk?.path, part?.name]); // al entrar

  const jump = async (p) => { setPath(p); await fetchList(p); };
  const goUp = async () => {
    if (path === "/") return;
    const parts = path.split("/").filter(Boolean);
    const p = parts.length <= 1 ? "/" : "/" + parts.slice(0, -1).join("/");
    await jump(p);
  };

  const openEntry = async (e) => {
    if (e.type === "dir") {
      const next = path === "/" ? `/${e.name}` : `${path}/${e.name}`;
      await jump(next);
      return;
    }
    try {
      const url = new URL(`${apiUrl}/viewer/file`);
      url.searchParams.set("diskPath", disk.path);
      url.searchParams.set("partName", part.name);
      const fpath = path === "/" ? `/${e.name}` : `${path}/${e.name}`;
      url.searchParams.set("path", fpath);
      const r = await fetch(url);
      if (!r.ok) throw new Error(`HTTP ${r.status}`);
      const json = await r.json();
      setFileMeta(json);
      setFileContent(json?.content || "");
      setOpenFile(true);
    } catch (er) {
      alert(er?.message || er);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <button onClick={goUp} className="px-3 py-1 border border-green-600 rounded text-green-400 hover:bg-gray-900" title="Subir nivel">⬆ Subir</button>
        <button onClick={() => fetchList()} className="px-3 py-1 border border-green-600 rounded text-green-400 hover:bg-gray-900" title="Actualizar">🔄 Actualizar</button>
        <div className="ml-2 flex items-center gap-2">
          <InfoPill>Disco: {disk.name}</InfoPill>
          <InfoPill>Partición: {part.name}</InfoPill>
        </div>
      </div>

      <div className="rounded-xl border border-green-600 p-6 bg-gray-950">
        <div className="flex items-center justify-between">
          <h3 className="text-xl font-bold text-green-400">Visualizador del Sistema de Archivos</h3>
          <Breadcrumbs path={path} onJump={jump} />
        </div>
        <p className="text-sm text-green-600 mt-1">Navegue entre carpetas o visualice archivos</p>

        <div className="mt-5">
          {loading && <div className="text-green-400">Cargando…</div>}
          {err && <div className="text-red-400">✖ {err}</div>}
          {!loading && !err && (
            <div className="flex flex-wrap gap-4">
              {entries.map((en) => <EntryCard key={`${en.inode}-${en.name}`} entry={en} onOpen={openEntry} />)}
              {entries.length === 0 && <div className="text-green-500 text-sm">Carpeta vacía</div>}
            </div>
          )}
        </div>
      </div>

      <FileViewerModal open={openFile} onClose={() => setOpenFile(false)} meta={fileMeta} content={fileContent} />
    </div>
  );
};

const ViewerTab = ({ apiUrl, apiStatus, viewerState, setViewerState }) => {
  const disk = viewerState.selectedDisk;
  const part = viewerState.selectedPart;

  if (!disk) {
    return <DiskSelector apiUrl={apiUrl} apiStatus={apiStatus} onSelected={(d) => setViewerState(v => ({ ...v, selectedDisk: d, selectedPart: null }))} />;
  }
  if (!part) {
    return (
      <PartitionSelector
        apiUrl={apiUrl}
        apiStatus={apiStatus}
        disk={disk}
        onBack={() => setViewerState(v => ({ ...v, selectedDisk: null, selectedPart: null }))}
        onSelected={(p) => setViewerState(v => ({ ...v, selectedPart: p }))}
      />
    );
  }
  return (
    <div className="space-y-6">
      <div className="flex items-center gap-3">
        <button onClick={() => setViewerState(v => ({ ...v, selectedPart: null }))} className="px-3 py-1 border border-green-600 rounded text-green-400 hover:bg-gray-900">← Cambiar partición</button>
        <InfoPill>{disk.name}</InfoPill>
        <InfoPill>{part.name}</InfoPill>
      </div>
      <Explorer apiUrl={apiUrl} disk={disk} part={part} />
    </div>
  );
};

/* ===========================
   ABOUT
   =========================== */
const AboutTab = () => (
  <div className="space-y-6">
    <div className="rounded-xl border border-green-600 p-6 bg-gray-950">
      <h2 className="text-2xl font-bold text-green-400">Acerca de</h2>
      <p className="text-green-500 mt-2 text-sm">GoDisk — Simulador de EXT2 en Go con interfaz web.</p>
    </div>
  </div>
);

/* ===========================
   APP PRINCIPAL
   =========================== */
export default function App() {
  const [activeTab, setActiveTab] = useState('console');

  // API
  const [apiUrl, setApiUrl] = useState("http://localhost:3001");
  const [apiStatus, setApiStatus] = useState('connecting');
  const checkApiStatus = useCallback(async () => {
    if (!apiUrl) { setApiStatus('offline'); return; }
    setApiStatus('connecting');
    try {
      const response = await fetch(`${apiUrl}/health`);
      setApiStatus(response.ok ? 'online' : 'offline');
    } catch { setApiStatus('offline'); }
  }, [apiUrl]);
  useEffect(() => { checkApiStatus(); }, [checkApiStatus]);

  // Sesión
  const [loginOpen, setLoginOpen] = useState(false);
  const [session, setSession] = useState({ loggedIn: false, user: "", id: "" });
  const handleLoggedIn = ({ user, id }) => setSession({ loggedIn: true, user, id });

  const handleLogout = async () => {
    if (apiStatus === 'online') {
      try {
        await fetch(`${apiUrl}/execute`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ command: "logout" }),
        });
      } catch { /* ignore */ }
    }
    setSession({ loggedIn: false, user: "", id: "" });
    window.dispatchEvent(new CustomEvent("gd-logout"));
  };

  // Visualizador
  const [viewerState, setViewerState] = useState({ selectedDisk: null, selectedPart: null });

  const renderActiveTab = () => {
    const props = { apiUrl, apiStatus };
    switch (activeTab) {
      case 'console': return <ConsoleTab {...props} session={session} />;
      case 'reports': return <ReportsTab {...props} session={session} />;
      case 'viewer': return <ViewerTab apiUrl={apiUrl} apiStatus={apiStatus} viewerState={viewerState} setViewerState={setViewerState} />;
      case 'about': return <AboutTab />;
      default: return <ConsoleTab {...props} session={session} />;
    }
  };

  return (
    <div className="min-h-screen bg-black text-green-400">
      <Header
        activeTab={activeTab}
        setActiveTab={setActiveTab}
        apiUrl={apiUrl}
        setApiUrl={setApiUrl}
        apiStatus={apiStatus}
        checkApiStatus={checkApiStatus}
        session={session}
        onOpenLogin={() => setLoginOpen(true)}
        onLogout={handleLogout}
      />
      <main className="max-w-6xl mx-auto px-6 py-8">
        {renderActiveTab()}
      </main>

      <LoginModal open={loginOpen} onClose={() => setLoginOpen(false)} apiUrl={apiUrl} apiStatus={apiStatus} onLoggedIn={handleLoggedIn} />
    </div>
  );
}
