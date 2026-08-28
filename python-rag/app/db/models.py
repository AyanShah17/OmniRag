import uuid
from datetime import datetime, timezone
from typing import Optional, List, Any
from sqlalchemy import (
    Column,
    String,
    Integer,
    BigInteger,
    Boolean,
    DateTime,
    ForeignKey,
    Text,
    JSON,
    Table,
    UniqueConstraint,
    CheckConstraint,
)
from sqlalchemy.orm import declarative_base, relationship

Base = declarative_base()


def generate_uuid() -> str:
    return str(uuid.uuid4())


def utc_now() -> datetime:
    return datetime.now(timezone.utc)


# Association table for Zero-Cost Chunk Re-linking across Document Versions
version_chunks = Table(
    "version_chunks",
    Base.metadata,
    Column("version_id", String(36), ForeignKey("document_versions.id", ondelete="CASCADE"), primary_key=True),
    Column("chunk_id", String(36), ForeignKey("chunks.id", ondelete="CASCADE"), primary_key=True),
    Column("chunk_order", Integer, default=0),
)


class Tenant(Base):
    __tablename__ = "tenants"

    id = Column(String(36), primary_key=True, default=generate_uuid)
    name = Column(String(255), nullable=False)
    plan = Column(String(50), default="free")
    created_at = Column(DateTime(timezone=True), default=utc_now)

    workspaces = relationship("Workspace", back_populates="tenant", cascade="all, delete-orphan")


class Workspace(Base):
    __tablename__ = "workspaces"

    id = Column(String(36), primary_key=True, default=generate_uuid)
    tenant_id = Column(String(36), ForeignKey("tenants.id", ondelete="CASCADE"), nullable=False)
    name = Column(String(255), nullable=False)
    created_at = Column(DateTime(timezone=True), default=utc_now)

    tenant = relationship("Tenant", back_populates="workspaces")
    connectors = relationship("Connector", back_populates="workspace", cascade="all, delete-orphan")
    documents = relationship("Document", back_populates="workspace", cascade="all, delete-orphan")
    conversations = relationship("Conversation", back_populates="workspace", cascade="all, delete-orphan")
    memberships = relationship("WorkspaceMembership", back_populates="workspace", cascade="all, delete-orphan")


class User(Base):
    __tablename__ = "users"

    id = Column(String(36), primary_key=True, default=generate_uuid)
    tenant_id = Column(String(36), ForeignKey("tenants.id", ondelete="CASCADE"), nullable=False)
    email = Column(String(255), unique=True, nullable=False)
    password_hash = Column(String(255), nullable=False)
    role = Column(String(50), default="member")  # admin, member, viewer
    created_at = Column(DateTime(timezone=True), default=utc_now)

    memberships = relationship("WorkspaceMembership", back_populates="user", cascade="all, delete-orphan")


class WorkspaceMembership(Base):
    """Explicit grant of a user's access to a workspace. This is the source of
    truth for tenant/workspace authorization checks (see app.core.authorization) —
    a caller presenting a valid identity for tenant A must never be able to read
    or write workspace data belonging to tenant B just by supplying a different
    X-Workspace-ID, and this table is what prevents that IDOR class of bug.
    """

    __tablename__ = "workspace_memberships"
    __table_args__ = (
        UniqueConstraint("user_id", "workspace_id", name="uq_membership_user_workspace"),
        CheckConstraint("role IN ('owner', 'admin', 'member', 'viewer')", name="ck_membership_role"),
    )

    id = Column(String(36), primary_key=True, default=generate_uuid)
    user_id = Column(String(36), ForeignKey("users.id", ondelete="CASCADE"), nullable=False)
    workspace_id = Column(String(36), ForeignKey("workspaces.id", ondelete="CASCADE"), nullable=False)
    tenant_id = Column(String(36), ForeignKey("tenants.id", ondelete="CASCADE"), nullable=False)
    role = Column(String(50), default="member")  # owner, admin, member, viewer
    created_at = Column(DateTime(timezone=True), default=utc_now)

    user = relationship("User", back_populates="memberships")
    workspace = relationship("Workspace", back_populates="memberships")


class Connector(Base):
    __tablename__ = "connectors"

    id = Column(String(36), primary_key=True, default=generate_uuid)
    workspace_id = Column(String(36), ForeignKey("workspaces.id", ondelete="CASCADE"), nullable=False)
    type = Column(String(50), nullable=False)  # s3, azure_blob, supabase_storage, confluence, local
    name = Column(String(255), nullable=False)
    config = Column(JSON, default=dict)
    is_active = Column(Boolean, default=True)
    sync_frequency = Column(String(50), default="hourly")  # realtime, hourly, daily, manual
    last_synced_at = Column(DateTime(timezone=True), nullable=True)
    created_at = Column(DateTime(timezone=True), default=utc_now)
    updated_at = Column(DateTime(timezone=True), default=utc_now, onupdate=utc_now)

    workspace = relationship("Workspace", back_populates="connectors")
    documents = relationship("Document", back_populates="connector", cascade="all, delete-orphan")


class Document(Base):
    __tablename__ = "documents"
    __table_args__ = (
        UniqueConstraint("workspace_id", "external_id", name="uq_document_workspace_external"),
        CheckConstraint(
            "status IN ('indexing', 'synced', 'error', 'deleting')",
            name="ck_document_status",
        ),
    )

    id = Column(String(36), primary_key=True, default=generate_uuid)
    workspace_id = Column(String(36), ForeignKey("workspaces.id", ondelete="CASCADE"), nullable=False)
    connector_id = Column(String(36), ForeignKey("connectors.id", ondelete="CASCADE"), nullable=True)
    external_id = Column(String(512), nullable=False)  # S3 Key, Azure URI, Confluence Page ID
    file_name = Column(String(512), nullable=False)
    file_type = Column(String(50), default="text/plain")
    file_size = Column(BigInteger, default=0)
    current_version_id = Column(String(36), nullable=True)
    status = Column(String(50), default="synced")  # pending, syncing, synced, error
    metadata_json = Column("metadata", JSON, default=dict)
    created_at = Column(DateTime(timezone=True), default=utc_now)
    updated_at = Column(DateTime(timezone=True), default=utc_now, onupdate=utc_now)

    workspace = relationship("Workspace", back_populates="documents")
    connector = relationship("Connector", back_populates="documents")
    versions = relationship("DocumentVersion", back_populates="document", cascade="all, delete-orphan")
    chunks = relationship("Chunk", back_populates="document", cascade="all, delete-orphan")


class DocumentVersion(Base):
    __tablename__ = "document_versions"
    __table_args__ = (UniqueConstraint("document_id", "version_number", name="uq_document_version_number"),)

    id = Column(String(36), primary_key=True, default=generate_uuid)
    document_id = Column(String(36), ForeignKey("documents.id", ondelete="CASCADE"), nullable=False)
    version_number = Column(Integer, nullable=False, default=1)
    file_hash = Column(String(64), nullable=False)  # SHA-256
    total_chunks = Column(Integer, default=0)
    created_at = Column(DateTime(timezone=True), default=utc_now)

    document = relationship("Document", back_populates="versions")
    chunks = relationship("Chunk", secondary=version_chunks, back_populates="versions")


class Chunk(Base):
    __tablename__ = "chunks"
    __table_args__ = (UniqueConstraint("document_id", "chunk_hash", name="uq_document_chunk_hash"),)

    id = Column(String(36), primary_key=True, default=generate_uuid)
    document_id = Column(String(36), ForeignKey("documents.id", ondelete="CASCADE"), nullable=False)
    chunk_hash = Column(String(64), nullable=False)  # SHA-256 of text
    chunk_index = Column(Integer, nullable=False, default=0)
    text_content = Column(Text, nullable=False)
    token_count = Column(Integer, default=0)
    metadata_json = Column("metadata", JSON, default=dict)
    is_embedded = Column(Boolean, default=False)
    created_at = Column(DateTime(timezone=True), default=utc_now)

    document = relationship("Document", back_populates="chunks")
    versions = relationship("DocumentVersion", secondary=version_chunks, back_populates="chunks")


class Conversation(Base):
    __tablename__ = "conversations"

    id = Column(String(36), primary_key=True, default=generate_uuid)
    workspace_id = Column(String(36), ForeignKey("workspaces.id", ondelete="CASCADE"), nullable=False)
    user_id = Column(String(36), nullable=True)
    title = Column(String(255), default="New Chat")
    created_at = Column(DateTime(timezone=True), default=utc_now)
    updated_at = Column(DateTime(timezone=True), default=utc_now, onupdate=utc_now)

    workspace = relationship("Workspace", back_populates="conversations")
    messages = relationship("Message", back_populates="conversation", cascade="all, delete-orphan")


class Message(Base):
    __tablename__ = "messages"
    __table_args__ = (
        CheckConstraint("role IN ('user', 'assistant', 'system')", name="ck_message_role"),
    )

    id = Column(String(36), primary_key=True, default=generate_uuid)
    conversation_id = Column(String(36), ForeignKey("conversations.id", ondelete="CASCADE"), nullable=False)
    role = Column(String(20), nullable=False)  # user, assistant, system
    content = Column(Text, nullable=False)
    citations = Column(JSON, default=list)  # Grounded citation cards with page, doc_id, snippet, link
    token_count = Column(Integer, default=0)
    created_at = Column(DateTime(timezone=True), default=utc_now)

    conversation = relationship("Conversation", back_populates="messages")


class SyncJob(Base):
    __tablename__ = "sync_jobs"
    __table_args__ = (
        CheckConstraint(
            "status IN ('pending', 'running', 'completed', 'failed')",
            name="ck_sync_job_status",
        ),
    )

    id = Column(String(36), primary_key=True, default=generate_uuid)
    connector_id = Column(String(36), ForeignKey("connectors.id", ondelete="CASCADE"), nullable=False)
    trigger_type = Column(String(64), nullable=False)
    status = Column(String(32), default="running")
    total_docs = Column(Integer, default=0)
    modified_docs = Column(Integer, default=0)
    embedded_chunks = Column(Integer, default=0)
    skipped_chunks = Column(Integer, default=0)
    error_log = Column(Text, nullable=True)
    started_at = Column(DateTime(timezone=True), default=utc_now)
    completed_at = Column(DateTime(timezone=True), nullable=True)


class AuditEvent(Base):
    """Immutable record of security-relevant actions: auth failures, document
    access, deletions, settings changes, and authorization denials. This is
    the trail an operator would need to answer "who did what, when, from
    where" after an incident — see app.core.audit for the writer helpers.
    Audit rows are append-only from the application's perspective; nothing in
    this codebase updates or deletes them.
    """

    __tablename__ = "audit_events"
    __table_args__ = (
        CheckConstraint("status IN ('success', 'denied', 'error')", name="ck_audit_status"),
    )

    id = Column(String(36), primary_key=True, default=generate_uuid)
    tenant_id = Column(String(36), nullable=True)
    workspace_id = Column(String(36), nullable=True)
    user_id = Column(String(36), nullable=True)
    action = Column(String(100), nullable=False)  # e.g. document.delete, auth.failed, settings.update
    resource_type = Column(String(50), nullable=True)  # e.g. document, conversation, settings
    resource_id = Column(String(36), nullable=True)
    status = Column(String(20), default="success")  # success, denied, error
    ip_address = Column(String(64), nullable=True)
    detail = Column(JSON, default=dict)
    created_at = Column(DateTime(timezone=True), default=utc_now)
