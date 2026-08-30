import logging
from sqlalchemy import inspect, text
from sqlalchemy.future import select
from app.db.session import engine, async_session_factory
from app.db.models import Base, Tenant, Workspace, User, WorkspaceMembership

logger = logging.getLogger("omnirag.db")


async def init_database():
    logger.info("Initializing database tables...")
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)

        def connector_columns(sync_conn):
            return {column["name"] for column in inspect(sync_conn).get_columns("connectors")}

        if "updated_at" not in await conn.run_sync(connector_columns):
            await conn.execute(text("ALTER TABLE connectors ADD COLUMN updated_at TIMESTAMP"))
            await conn.execute(text("UPDATE connectors SET updated_at = created_at WHERE updated_at IS NULL"))
    logger.info("Database schema initialized successfully.")

    # Seed default tenant and workspace if not present
    async with async_session_factory() as session:
        result = await session.execute(select(Tenant).where(Tenant.id == "tenant_default"))
        tenant = result.scalar_one_or_none()
        if not tenant:
            tenant = Tenant(
                id="tenant_default",
                name="Enterprise Demo Organization",
                plan="enterprise",
            )
            session.add(tenant)

        workspace_result = await session.execute(select(Workspace).where(Workspace.id == "ws_default"))
        if workspace_result.scalar_one_or_none() is None:
            session.add(Workspace(
                id="ws_default",
                tenant_id="tenant_default",
                name="Main Knowledge Base",
            ))

        admin_result = await session.execute(select(User).where(User.id == "user_admin"))
        if admin_result.scalar_one_or_none() is None:
            session.add(User(
                id="user_admin",
                tenant_id="tenant_default",
                email="admin@omnirag.ai",
                password_hash="$2b$12$dummyhashforlocalmvpadminuser2026",
                role="admin",
            ))

            # The dev-mode auth fallback identity (see app.api.v1.auth) also needs an
            # explicit membership row so authorization checks succeed for local/test
            # runs without weakening the production authorization path.
        dev_result = await session.execute(select(User).where(User.id == "user_dev_enterprise"))
        if dev_result.scalar_one_or_none() is None:
            session.add(User(
                id="user_dev_enterprise",
                tenant_id="tenant_default",
                email="dev@omnirag.ai",
                password_hash="$2b$12$dummyhashforlocaldevuser2026xx",
                role="admin",
            ))

        await session.flush()

        membership_specs = (
            ("membership_admin_default", "user_admin", "owner"),
            ("membership_dev_default", "user_dev_enterprise", "admin"),
        )
        for membership_id, user_id, role in membership_specs:
            membership_result = await session.execute(
                select(WorkspaceMembership).where(WorkspaceMembership.id == membership_id)
            )
            if membership_result.scalar_one_or_none() is None:
                session.add(WorkspaceMembership(
                    id=membership_id,
                    user_id=user_id,
                    workspace_id="ws_default",
                    tenant_id="tenant_default",
                    role=role,
                ))

        await session.commit()
        logger.info("Default tenant, workspace, users, and memberships are ready.")
