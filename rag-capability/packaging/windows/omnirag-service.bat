@echo off
TITLE OmniRAG Dynamic Enterprise RAG Backend
echo ======================================================================
echo           OmniRAG Dynamic Enterprise RAG - Backend Service
echo ======================================================================
echo.

SET SCRIPT_DIR=%~dp0
SET OMNIRAG_CONFIG=C:\ProgramData\OmniRAG\omnirag.env

IF NOT EXIST "%OMNIRAG_CONFIG%" (
    echo [NOTICE] Configuration file not found. Running initial setup wizard...
    python "%SCRIPT_DIR%omnirag_config.py" --config "%OMNIRAG_CONFIG%"
)

echo Starting Go Connector Engine (Port 8080)...
start "OmniRAG Go Connector Engine" "%SCRIPT_DIR%go-engine.exe"

echo Starting Python AI & Vector Core (Port 8000)...
start "OmniRAG Python RAG Core" python -m uvicorn app.main:app --host 0.0.0.0 --port 8000 --app-dir "%SCRIPT_DIR%..\python-rag"

timeout /t 3 /nobreak >nul
echo.
echo OmniRAG is running!
echo Access Web UI at: http://localhost:8000
echo.
start http://localhost:8000
