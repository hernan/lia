##Feature Admin Panel
The admin panel is a feature that allows logged in users to see and manage generated shortened URLs.
The feature is a kind of administrative panel, is not meant to be used by the general public.

###Accessing the Admin Panel
To access the admin panel, users must be logged in. Once logged in, they are redirected to the admin panel where they can see all the generated shortened URLs.
The list of URLs is displayed in a table format, showing the original URL, the shortened URL, and the date it was created.
Users can also search for specific URLs using a search bar.
At all times the list is sorted by the date of creation, with the most recent URLs appearing at the top of the list.

###Managing Shortened URLs
In the admin panel, users have the ability to manage their shortened URLs. They can delete any URL they no longer need by clicking on the delete button next to the URL in the table. This action will remove the URL from the list and it will no longer be accessible through the shortened link.
Users can also edit the original URL associated with a shortened URL. By clicking on the edit button next to the URL, they can update the original URL and save the changes. This allows users to keep their shortened URLs up to date without having to create new ones.
Overall, the admin panel provides users with a convenient way to view and manage their generated shortened URLs.

##Security Considerations
The admin panel is only accessible to logged in users, which helps to ensure that only authorized individuals can view and manage the shortened URLs. However, it is important to implement additional security measures to protect the admin panel from unauthorized access.

##Implementation Details
The admin panel is implemented using a combination of frontend and backend technologies. 
The frontend is built using HTML, CSS, and JavaScript to create a user-friendly interface for using the TMPL go library, the rest of the logic will be implemented in GO from the structure already present in the project.

###PLAN
1. Create a new route in the backend to handle requests to the admin panel.
2. Implement authentication and authorization checks to ensure that only logged in users can access the admin panel.
3. Create a new HTML template for the admin panel that displays the list of shortened URLs in a table format.
4. Create a new HTML template for the edit URL functionality that allows users to update the original URL associated with a shortened URL.
5. Implement functionality to search for specific URLs in the admin panel using a search bar.
6. Implement functionality to allow users to delete and edit their shortened URLs from the admin panel.
7. Test the admin panel to ensure that it is functioning correctly and securely.

Break down the implementation from the plan into smaller tasks if necessary, make sure to produce atomic units of work with corresponding tests to ensure the functionality is working as expected.
