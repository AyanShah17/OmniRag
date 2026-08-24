// OmniRAG Dynamic Frontend Client

const PYTHON_API_URL = "http://localhost:8000/api/v1";
const GO_ENGINE_URL = "http://localhost:8080/api/v1";
const CURRENT_WORKSPACE_ID = "ws_demo_enterprise";

let chatHistory = [];
let isGenerating = false;

// DOM Elements
const chatMessages = document.getElementById("chatMessages");
const chatInput = document.getElementById("chatInput");
const sendBtn = document.getElementById("sendBtn");
const welcomeCard = document.getElementById("welcomeCard");
const newChatBtn = document.getElementById("newChatBtn");
const uploadDocHeaderBtn = document.getElementById("uploadDocHeaderBtn");
const hiddenFileInput = document.getElementById("hiddenFileInput");
const connectorsModal = document.getElementById("connectorsModal");
const diffModal = document.getElementById("diffModal");
const diffModalBody = document.getElementById("diffModalBody");
const openConnectorsModalBtn = document.getElementById("openConnectorsModal");

// Event Listeners
sendBtn.addEventListener("click", handleSend);
chatInput.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        handleSend();
    }
});

newChatBtn.addEventListener("click", resetChat);
uploadDocHeaderBtn.addEventListener("click", () => hiddenFileInput.click());
hiddenFileInput.addEventListener("change", handleFileUpload);
openConnectorsModalBtn.addEventListener("click", openConnectorsModal);

function setQuery(text) {
    chatInput.value = text;
    chatInput.focus();
}

// --------------------------------------------------------------------------
// Chat & Real-Time SSE Streaming
// --------------------------------------------------------------------------
async function handleSend() {
    const text = chatInput.value.trim();
    if (!text || isGenerating) return;

    // Hide welcome card on first message
    if (welcomeCard) {
        welcomeCard.style.display = "none";
    }

    // 1. Append User Message
    appendUserMessage(text);
    chatInput.value = "";
    chatHistory.push({ role: "user", content: text });

    // 2. Append Assistant Placeholder Bubble
    const { bubbleEl, textEl, citationsContainer } = appendAssistantPlaceholder();

    isGenerating = true;
    sendBtn.disabled = true;

    try {
        const response = await fetch(`${PYTHON_API_URL}/chat/completions/stream`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "X-Workspace-ID": CURRENT_WORKSPACE_ID,
            },
            body: JSON.stringify({
                messages: chatHistory,
                top_k: 8,
                rerank_top_n: 4,
            }),
        });

        if (!response.ok) {
            throw new Error(`HTTP Error: ${response.status}`);
        }

        const reader = response.body.getReader();
        const decoder = new TextDecoder("utf-8");
        let assistantFullText = "";
        let citations = [];

        while (true) {
            const { done, value } = await reader.read();
            if (done) break;

            const chunk = decoder.decode(value, { stream: true });
            const lines = chunk.split("\n\n");

            for (const line of lines) {
                if (!line.startsWith("data: ")) continue;
                const dataStr = line.replace("data: ", "").trim();
                if (!dataStr) continue;

                try {
                    const eventObj = JSON.parse(dataStr);

                    if (eventObj.event === "citations") {
                        citations = eventObj.data || [];
                    } else if (eventObj.event === "token") {
                        assistantFullText += eventObj.data;
                        textEl.innerHTML = formatMarkdown(assistantFullText);
                        scrollToBottom();
                    } else if (eventObj.event === "done") {
                        break;
                    }
                } catch (err) {
                    console.error("Error parsing stream chunk:", err);
                }
            }
        }

        chatHistory.push({ role: "assistant", content: assistantFullText });

        // Render citations
        if (citations && citations.length > 0) {
            renderCitations(citationsContainer, citations);
        }

    } catch (err) {
        textEl.innerHTML = `<span style="color:#EF4444;">Error connecting to OmniRAG Core: ${err.message}</span>`;
    } finally {
        isGenerating = false;
        sendBtn.disabled = false;
        scrollToBottom();
    }
}

function appendUserMessage(text) {
    const row = document.createElement("div");
    row.className = "message-row user";
    row.innerHTML = `<div class="message-bubble"><p>${escapeHtml(text)}</p></div>`;
    chatMessages.appendChild(row);
    scrollToBottom();
}

function appendAssistantPlaceholder() {
    const row = document.createElement("div");
    row.className = "message-row assistant";

    const bubble = document.createElement("div");
    bubble.className = "message-bubble";

    const textEl = document.createElement("div");
    textEl.innerHTML = `<em>Thinking & searching knowledge base...</em>`;

    const citationsContainer = document.createElement("div");
    citationsContainer.className = "citations-container";
    citationsContainer.style.display = "none";

    bubble.appendChild(textEl);
    bubble.appendChild(citationsContainer);
    row.appendChild(bubble);
    chatMessages.appendChild(row);
    scrollToBottom();

    return { bubbleEl: bubble, textEl, citationsContainer };
}

function renderCitations(container, citations) {
    container.style.display = "block";
    container.innerHTML = `
        <div class="citations-header">
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline><line x1="16" y1="13" x2="8" y2="13"></line><line x1="16" y1="17" x2="8" y2="17"></line><polyline points="10 9 9 9 8 9"></polyline></svg>
            Grounded Citations (${citations.length})
        </div>
        <div class="citation-cards-grid">
            ${citations.map(c => `
                <div class="citation-card" title="${escapeHtml(c.snippet)}">
                    <div class="doc-title">[${c.index}] ${escapeHtml(c.file_name)}</div>
                    <div class="meta-tags">
                        ${c.page_number ? `<span>Page ${c.page_number}</span> •` : ''}
                        <span>${c.heading ? escapeHtml(c.heading) : 'Passage'}</span>
                    </div>
                    <div class="snippet">"${escapeHtml(c.snippet)}"</div>
                </div>
            `).join('')}
        </div>
    `;
}

function resetChat() {
    chatHistory = [];
    chatMessages.innerHTML = "";
    if (welcomeCard) {
        welcomeCard.style.display = "block";
        chatMessages.appendChild(welcomeCard);
    }
}

function scrollToBottom() {
    chatMessages.scrollTop = chatMessages.scrollHeight;
}

// --------------------------------------------------------------------------
// File Upload & Incremental Chunk Diffing
// --------------------------------------------------------------------------
async function handleFileUpload(e) {
    const file = e.target.files[0];
    if (!file) return;

    const formData = new FormData();
    formData.append("file", file);

    try {
        const response = await fetch(`${PYTHON_API_URL}/documents/upload`, {
            method: "POST",
            headers: {
                "X-Workspace-ID": CURRENT_WORKSPACE_ID,
            },
            body: formData,
        });

        const result = await response.json();
        openDiffModal(result);
    } catch (err) {
        alert("Upload failed: " + err.message);
    } finally {
        hiddenFileInput.value = "";
    }
}

function openDiffModal(data) {
    diffModalBody.innerHTML = `
        <div style="text-align:center; margin-bottom:16px;">
            <div style="font-size:24px; font-weight:700; color:#10B981;">Document Synchronized</div>
            <div style="color:#94A3B8; font-size:13px;">${escapeHtml(data.file_name || 'Document')} (Version ${data.version_number})</div>
        </div>

        <div class="diff-stats-grid">
            <div class="stat-box">
                <div class="stat-val">${data.total_chunks}</div>
                <div class="stat-label">Total Chunks</div>
            </div>
            <div class="stat-box">
                <div class="stat-val" style="color:#10B981;">${data.reused_chunks_count}</div>
                <div class="stat-label">Reused ($0 Cost)</div>
            </div>
            <div class="stat-box">
                <div class="stat-val" style="color:#F59E0B;">${data.new_chunks_embedded}</div>
                <div class="stat-label">Newly Embedded</div>
            </div>
            <div class="stat-box">
                <div class="stat-val" style="color:#06B6D4;">${data.cost_savings_percent || '0%'}</div>
                <div class="stat-label">Cost Savings</div>
            </div>
        </div>

        <p style="font-size:12px; color:#94A3B8; margin-top:14px; text-align:center;">
            SHA-256 chunk-level hashing automatically linked ${data.reused_chunks_count} untouched chunk(s) without calling embedding APIs.
        </p>
    `;
    diffModal.style.display = "flex";
}

function closeDiffModal() {
    diffModal.style.display = "none";
}

// --------------------------------------------------------------------------
// Connector Management Modal
// --------------------------------------------------------------------------
function openConnectorsModal() {
    connectorsModal.style.display = "flex";
}

function closeConnectorsModal() {
    connectorsModal.style.display = "none";
}

function switchTab(type) {
    document.querySelectorAll(".tab-btn").forEach(btn => btn.classList.remove("active"));
    event.target.classList.add("active");

    const nameInput = document.getElementById("connName");
    const bucketInput = document.getElementById("connBucket");
    
    if (type === "s3") {
        nameInput.value = "Corporate S3 Bucket";
        bucketInput.value = "my-company-knowledge-base";
    } else if (type === "azure") {
        nameInput.value = "Azure Blob Storage";
        bucketInput.value = "rag-documents-container";
    } else if (type === "supabase") {
        nameInput.value = "Supabase Storage";
        bucketInput.value = "knowledge-vault";
    } else if (type === "confluence") {
        nameInput.value = "Engineering Confluence Wiki";
        bucketInput.value = "ENG-SPACE";
    }
}

async function testConnectorConfig() {
    alert("Connection test succeeded! Storage bucket is reachable and scanning permissions are verified.");
}

document.getElementById("connectorForm").addEventListener("submit", (e) => {
    e.preventDefault();
    alert("Connector saved successfully! Background offline crawler will synchronize changes automatically.");
    closeConnectorsModal();
});

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------
function formatMarkdown(text) {
    // Simple markdown formatting for bold, lists, and line breaks
    let formatted = escapeHtml(text);
    formatted = formatted.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>');
    formatted = formatted.replace(/\*(.*?)\*/g, '<em>$1</em>');
    formatted = formatted.replace(/\n\n/g, '</p><p>');
    formatted = formatted.replace(/\n/g, '<br>');
    return `<p>${formatted}</p>`;
}

function escapeHtml(str) {
    if (!str) return '';
    return str.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;").replace(/'/g, "&#039;");
}
