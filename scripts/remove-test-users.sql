-- scripts/remove-test-users.sql

-- Удаляем сообщения тестовых пользователей
DELETE FROM messages 
WHERE user_id IN (SELECT id FROM users WHERE username IN ('test', 'admin'));

-- Удаляем самих тестовых пользователей
DELETE FROM users 
WHERE username IN ('test', 'admin');

-- Проверяем результат
SELECT COUNT(*) as remaining_users FROM users;
SELECT COUNT(*) as remaining_messages FROM messages;