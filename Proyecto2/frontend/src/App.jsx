import React, { useEffect, useRef, useState, useCallback } from "react";

// --- NUEVO --- Componente para la conexión de la API
const ApiConnector = ({ apiUrl, setApiUrl, apiStatus, checkApiStatus }) => {
  const statusMap = {
    online: { text: "Conectado", color: "bg-green-500", textColor: "text-green-400" },
    offline: { text: "Desconectado", color: "bg-red-500", textColor: "text-red-400" },
    connecting: { text: "Conectando...", color: "bg-yellow-500", textColor: "text-yellow-400" },
  };

  const currentStatus = statusMap[apiStatus] || statusMap.offline;

  return (
    <div className="flex items-center gap-4 p-3 bg-black border border-green-700 rounded-lg">
      <label htmlFor="api-url" className="text-green-400 font-semibold">
        API Backend:
      </label>
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


// Componente de Header con navegación (modificado)
const Header = ({ activeTab, setActiveTab, apiUrl, setApiUrl, apiStatus, checkApiStatus }) => {
  const tabs = [
    { id: 'console', label: 'Consola', icon: '💻' },
    { id: 'reports', label: 'Reportes', icon: '📊' },
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
        </div>
        
        {/* --- NUEVO --- Se añade el conector de API aquí */}
        <ApiConnector 
            apiUrl={apiUrl}
            setApiUrl={setApiUrl}
            apiStatus={apiStatus}
            checkApiStatus={checkApiStatus}
        />

        <nav className="flex space-x-1">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex items-center space-x-2 px-4 py-2 rounded-lg transition ${
                activeTab === tab.id
                  ? 'bg-green-500 text-black font-semibold'
                  : 'text-green-400 hover:bg-gray-800 hover:text-green-300'
              }`}
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

// Componente de Menú de Archivo (sin cambios)
const FileMenu = ({ onOpenFile, onSaveFile, onSaveAsFile, currentFileName }) => {
  const [isOpen, setIsOpen] = useState(false);

  return (
    <div className="relative">
      <button
        onClick={() => setIsOpen(!isOpen)}
        className="flex items-center space-x-2 px-4 py-2 bg-gray-900 text-green-400 border border-green-500 rounded hover:bg-green-500 hover:text-black transition"
      >
        <span>📄</span>
        <span>Archivo</span>
      </button>
      
      {isOpen && (
        <>
          <div 
            className="fixed inset-0 z-10" 
            onClick={() => setIsOpen(false)}
          />
          <div className="absolute top-full left-0 mt-1 bg-black border border-green-500 rounded-lg shadow-lg z-20 min-w-48">
            <button
              onClick={() => { onOpenFile(); setIsOpen(false); }}
              className="w-full flex items-center space-x-2 px-4 py-2 text-left text-green-400 hover:bg-gray-800 rounded-t-lg"
            >
              <span>📂</span>
              <span>Abrir archivo...</span>
            </button>
            <button
              onClick={() => { onSaveFile(); setIsOpen(false); }}
              disabled={!currentFileName}
              className="w-full flex items-center space-x-2 px-4 py-2 text-left text-green-400 hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <span>💾</span>
              <span>Guardar</span>
              {currentFileName && <span className="text-xs text-green-600">({currentFileName})</span>}
            </button>
            <button
              onClick={() => { onSaveAsFile(); setIsOpen(false); }}
              className="w-full flex items-center space-x-2 px-4 py-2 text-left text-green-400 hover:bg-gray-800 rounded-b-lg"
            >
              <span>⬇️</span>
              <span>Guardar como...</span>
            </button>
          </div>
        </>
      )}
    </div>
  );
};

// Componente de la Consola (modificado)
const ConsoleTab = ({ apiUrl, apiStatus }) => {
  const [entradaComandos, setEntradaComandos] = useState("");
  const [salidaComandos, setSalidaComandos] = useState("");
  const [loading, setLoading] = useState(false);
  const [currentFileName, setCurrentFileName] = useState("");
  const outRef = useRef(null);
  const fileInputRef = useRef(null);

  useEffect(() => {
    outRef.current?.scrollTo({ top: outRef.current.scrollHeight });
  }, [salidaComandos, loading]);

  const manejarEjecucion = async () => {
    const command = entradaComandos.trim();
    if (!command || loading || apiStatus !== 'online') return;

    try {
      setLoading(true);
      setSalidaComandos((prev) => prev + `\n▶ Ejecutando...\n`);
      
      // --- MODIFICADO --- Usa la URL de la API del estado global
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
      if ((e.ctrlKey || e.metaKey) && e.key === "Enter") {
        e.preventDefault();
        manejarEjecucion();
      }
    };
    window.addEventListener("keydown", h);
    return () => window.removeEventListener("keydown", h);
  }, [entradaComandos, loading, apiStatus]); // <-- Añadido para que el atajo respete el estado

  // ... (resto de funciones de ConsoleTab sin cambios)
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
    const a = document.createElement('a');
    a.href = url;
    a.download = currentFileName;
    a.click();
    URL.revokeObjectURL(url);
  };
  const handleSaveAsFile = () => {
    const fileName = prompt('Nombre del archivo:', currentFileName || 'script.smia');
    if (!fileName) return;
    const blob = new Blob([entradaComandos], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = fileName;
    a.click();
    URL.revokeObjectURL(url);
    setCurrentFileName(fileName);
  };
  const limpiarSalida = () => setSalidaComandos("");
  const copiarSalida = async () => {
    try { await navigator.clipboard.writeText(salidaComandos); alert('Salida copiada al portapapeles'); } catch { alert('Error al copiar al portapapeles'); }
  };
  const onDrop = async (e) => {
    e.preventDefault();
    const file = e.dataTransfer.files?.[0];
    if (!file) return;
    setEntradaComandos(await file.text());
    setCurrentFileName(file.name);
  };
  const onDragOver = (e) => e.preventDefault();

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center gap-3 p-4 bg-gray-900 border border-green-500 rounded-lg">
        <FileMenu onOpenFile={handleOpenFile} onSaveFile={handleSaveFile} onSaveAsFile={handleSaveAsFile} currentFileName={currentFileName} />
        <div className="w-px h-8 bg-green-500" />
        <button
          onClick={manejarEjecucion}
          // --- MODIFICADO --- Deshabilitado si no hay conexión
          disabled={loading || !entradaComandos.trim() || apiStatus !== 'online'}
          className={`flex items-center space-x-2 px-4 py-2 rounded transition ${
            loading || apiStatus !== 'online'
              ? "bg-gray-800 text-gray-500 cursor-not-allowed"
              : "bg-green-500 text-black font-semibold hover:bg-green-400"
          }`}
        >
          <span>{loading ? "⏳" : "▶️"}</span>
          <span>{loading ? "Ejecutando..." : "Ejecutar"}</span>
        </button>
        {apiStatus !== 'online' && <span className="text-xs text-red-400">API Desconectada</span>}
        <p className="text-xs text-green-600 ml-auto">
          <kbd className="px-2 py-1 bg-black border border-green-500 rounded text-xs text-green-400">Ctrl/Cmd + Enter</kbd> para ejecutar
        </p>
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
          <button onClick={copiarSalida} disabled={!salidaComandos} className="px-3 py-1 text-sm border border-green-500 text-green-400 rounded hover:bg-green-500 hover:text-black transition disabled:opacity-50">
            📋 Copiar
          </button>
          <button onClick={limpiarSalida} disabled={!salidaComandos} className="px-3 py-1 text-sm border border-red-500 text-red-400 rounded hover:bg-red-500 hover:text-black transition disabled:opacity-50">
            🗑️ Limpiar
          </button>
        </div>
      </div>
      
      <div ref={outRef} className="bg-black border border-green-500 rounded-lg p-4 h-80 overflow-auto text-green-400 font-mono">
        <pre>{salidaComandos || "# Salida de comandos aparecerá aquí..."}</pre>
      </div>

      <input ref={fileInputRef} type="file" accept=".smia,.txt" onChange={manejarArchivo} className="hidden" />
    </div>
  );
};

// Componente de Reportes (modificado para recibir props)
const ReportsTab = ({ apiUrl, apiStatus }) => {
  const [selectedReport, setSelectedReport] = useState(null);
  // ... (código interno de ReportsTab sin cambios)
  const reports = [
    { id: 'mbr', name: 'Reporte MBR', description: 'Visualizar Master Boot Record', icon: '💾', color: 'bg-green-600' },
    { id: 'disk', name: 'Reporte de Disco', description: 'Estructura completa del disco', icon: '💿', color: 'bg-green-500' },
    { id: 'inode', name: 'Reporte de Inodos', description: 'Información de inodos del sistema', icon: '🗂️', color: 'bg-green-700' },
    { id: 'journaling', name: 'Reporte de Journaling', description: 'Registro de transacciones', icon: '📋', color: 'bg-green-600' },
    { id: 'tree', name: 'Árbol de Directorios', description: 'Estructura jerárquica de archivos', icon: '🌳', color: 'bg-green-500' },
    { id: 'sb', name: 'Super Bloque', description: 'Información del sistema de archivos', icon: '🧱', color: 'bg-green-700' }
  ];
  const generateReport = (reportType) => {
    if (apiStatus !== 'online') {
        alert('Error: No se puede generar el reporte. La API está desconectada.');
        return;
    }
    alert(`Generando reporte: ${reportType} desde ${apiUrl}\n\nEsta funcionalidad se conectará con el backend.`);
  };

  return (
    <div className="space-y-6">
       {/* ... (código JSX de ReportsTab sin cambios) */}
    </div>
  );
};

// Componente Acerca de (sin cambios)
const AboutTab = () => {
    // ... (código interno y JSX de AboutTab sin cambios)
    return (
        <div className="space-y-6">
            {/* ... */}
        </div>
    );
};


// --- MODIFICADO --- Componente principal
export default function App() {
  const [activeTab, setActiveTab] = useState('console');
  
  // --- NUEVO --- Estado para la API
  const [apiUrl, setApiUrl] = useState("http://localhost:3001");
  const [apiStatus, setApiStatus] = useState('connecting'); // 'online', 'offline', 'connecting'

  // --- NUEVO --- Función para verificar el estado de la API
  const checkApiStatus = useCallback(async () => {
    if (!apiUrl) {
      setApiStatus('offline');
      return;
    }
    setApiStatus('connecting');
    try {
      const response = await fetch(`${apiUrl}/health`);
      if (response.ok) {
        setApiStatus('online');
      } else {
        setApiStatus('offline');
      }
    } catch (error) {
      setApiStatus('offline');
    }
  }, [apiUrl]);

  // --- NUEVO --- Verificar estado al cargar la página
  useEffect(() => {
    checkApiStatus();
  }, [checkApiStatus]);

  const renderActiveTab = () => {
    const props = { apiUrl, apiStatus };
    switch (activeTab) {
      case 'console':
        return <ConsoleTab {...props} />;
      case 'reports':
        return <ReportsTab {...props} />;
      case 'about':
        return <AboutTab />;
      default:
        return <ConsoleTab {...props} />;
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
      />
      <main className="max-w-6xl mx-auto px-6 py-8">
        {renderActiveTab()}
      </main>
    </div>
  );
}